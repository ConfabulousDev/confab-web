// Package pricingsource owns the model price table: the compiled-in default
// (embedded pricing.json — the single source of truth in the repo) plus an
// optional best-effort refresh from a remote URL (confabulous.dev). It serves
// the /api/v1/pricing endpoint and feeds the analytics cost compute.
//
// Modeled on internal/updatecheck: a lazy TTL-cached fetch, never blocking,
// always returning a valid document (the embedded floor at worst).
package pricingsource

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// maxSchemaVersion is the highest document schema this build understands. A
// remote document advertising a higher version is a format we don't trust and
// is rejected (→ fall back to embedded). Reserved at 0; bump only on a
// structural format change.
const maxSchemaVersion = 0

// defaultSourceURL is where a self-hosted backend pulls the freshest table when
// PRICING_SOURCE_URL is unset. The canonical SaaS instance disables fetching
// (empty URL) so it never requests from itself.
const defaultSourceURL = "https://confabulous.dev/api/v1/pricing"

// maxBodyBytes bounds the fetched response so a misbehaving source can't make
// us read an unbounded body. The real document is a few KB.
const maxBodyBytes = 1 << 20 // 1 MiB

// Tunables are vars (not consts) so tests can shrink them.
var (
	successTTL     = 2 * time.Hour
	failureTTL     = 15 * time.Minute
	requestTimeout = 3 * time.Second
)

// Rate is the per-million-token price for one model family (USD).
type Rate struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cacheWrite"`
	CacheRead  float64 `json:"cacheRead"`
}

// Document is the versioned price table, provider-nested (provider → family →
// rate) to match the frontend shape. It is the wire shape of /api/v1/pricing.
type Document struct {
	SchemaVersion int                        `json:"schema_version"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	Pricing       map[string]map[string]Rate `json:"pricing"`
}

//go:embed pricing.json
var embeddedJSON []byte
var embeddedDoc Document

func init() {
	if err := json.Unmarshal(embeddedJSON, &embeddedDoc); err != nil {
		panic("pricingsource: embedded pricing.json is unparseable: " + err.Error())
	}
	// A broken compiled-in artifact is a programming error, not a runtime
	// condition — fail loudly at startup rather than silently serving garbage.
	if err := validate(embeddedDoc); err != nil {
		panic("pricingsource: embedded pricing.json is invalid: " + err.Error())
	}
}

// Embedded returns the compiled-in floor table.
func Embedded() Document { return embeddedDoc }

// Source manages the embedded floor + a lazily-refreshed remote table.
type Source struct {
	embedded Document
	url      string        // empty ⇒ disabled (never egress)
	refresh  time.Duration // success TTL; also drives Cache-Control max-age
	client   *http.Client

	mu        sync.Mutex
	cached    *Document // last good remote fetch (nil until first success)
	fetchedAt time.Time
	cacheTTL  time.Duration // refresh on success, failureTTL on miss
}

// NewSource binds a source to an embedded floor and an optional remote URL.
// An empty url disables fetching entirely.
func NewSource(embedded Document, url string, refresh time.Duration) *Source {
	return &Source{
		embedded: embedded,
		url:      url,
		refresh:  refresh,
		client:   &http.Client{Timeout: requestTimeout},
	}
}

// NewFromEnv builds a source from the embedded floor and PRICING_SOURCE_URL /
// PRICING_REFRESH_INTERVAL. forceDisabled blanks the URL (the composition root
// passes this when running as SaaS, so confabulous.dev serves its own table).
func NewFromEnv(forceDisabled bool) *Source {
	url := envSourceURL()
	if forceDisabled {
		url = ""
	}
	return NewSource(Embedded(), url, envRefreshInterval())
}

// RefreshInterval is the success TTL, exposed for the endpoint's Cache-Control.
func (s *Source) RefreshInterval() time.Duration { return s.refresh }

// Effective returns the freshest valid table: the remote document when it is
// reachable, valid, and strictly newer than the embedded floor; otherwise the
// embedded floor (or the last-good remote). Lazy: refreshes the cache on call
// when stale, never blocking on a background loop. Never errors — the embedded
// floor is always a valid fallback.
func (s *Source) Effective(ctx context.Context) Document {
	if s.url == "" {
		return s.embedded // disabled: never egress
	}

	s.mu.Lock()
	if !s.fetchedAt.IsZero() && time.Since(s.fetchedAt) < s.cacheTTL {
		cached := s.cached
		s.mu.Unlock()
		return s.freshest(cached)
	}
	s.mu.Unlock()

	// Fetch outside the lock so concurrent callers don't queue on the network.
	// Two concurrent stale-cache callers may both fetch; accepted per design.
	doc, err := s.fetch(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		logger.Warn("pricing source fetch failed; using last-good/embedded", "error", err, "url", s.url)
		s.fetchedAt = time.Now()
		s.cacheTTL = failureTTL
		return s.freshest(s.cached) // keep whatever we last had
	}
	s.cached = &doc
	s.fetchedAt = time.Now()
	s.cacheTTL = s.refresh
	return s.freshest(s.cached)
}

// freshest picks the remote table only when it is strictly newer than the
// embedded floor; ties and older remotes fall to embedded.
func (s *Source) freshest(remote *Document) Document {
	if remote != nil && remote.UpdatedAt.After(s.embedded.UpdatedAt) {
		return *remote
	}
	return s.embedded
}

func (s *Source) fetch(ctx context.Context) (Document, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, s.url, nil)
	if err != nil {
		return Document{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return Document{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Document{}, fmt.Errorf("pricing source returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return Document{}, err
	}

	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return Document{}, err
	}
	if err := validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

// validate reports whether a document is safe to trust: a known schema, a
// non-zero timestamp, a non-empty table, and finite non-negative rates.
func validate(d Document) error {
	if d.SchemaVersion < 0 || d.SchemaVersion > maxSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d", d.SchemaVersion)
	}
	if d.UpdatedAt.IsZero() {
		return errors.New("updated_at is zero")
	}
	if len(d.Pricing) == 0 {
		return errors.New("pricing table is empty")
	}
	families := 0
	for _, fams := range d.Pricing {
		for family, r := range fams {
			for _, v := range []float64{r.Input, r.Output, r.CacheWrite, r.CacheRead} {
				if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
					return fmt.Errorf("invalid rate for family %q", family)
				}
			}
			families++
		}
	}
	if families == 0 {
		return errors.New("pricing table has no families")
	}
	return nil
}

// expireCache is a test-only seam that backdates the cached fetch time so the
// next Effective() refetches.
func expireCache(s *Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetchedAt = time.Time{}
}

// envSourceURL resolves PRICING_SOURCE_URL: present (even empty) wins as-is so
// an operator can disable fetching with PRICING_SOURCE_URL=""; absent falls
// back to the canonical default.
func envSourceURL() string {
	if v, ok := os.LookupEnv("PRICING_SOURCE_URL"); ok {
		return v
	}
	return defaultSourceURL
}

// envRefreshInterval parses PRICING_REFRESH_INTERVAL as a Go duration; unset,
// unparseable, or non-positive falls back to the success TTL.
func envRefreshInterval() time.Duration {
	if v := os.Getenv("PRICING_REFRESH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return successTTL
}
