package pricingsource

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var (
	embeddedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newerAt    = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	olderAt    = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

// testEmbedded is a small, valid floor used by Effective tests (independent of
// the real pricing.json so freshest-wins assertions are stable).
func testEmbedded() Document {
	return Document{
		SchemaVersion: 0,
		UpdatedAt:     embeddedAt,
		Pricing: map[string]map[string]Rate{
			"claude-code": {"opus-4-7": {Input: 5, Output: 25, CacheWrite: 6.25, CacheRead: 0.5}},
		},
	}
}

// remoteDoc builds a remote payload whose opus-4-7 input rate is `input` (so a
// test can tell remote from embedded by reading that one number).
func remoteDoc(updated time.Time, schema int, input float64) Document {
	return Document{
		SchemaVersion: schema,
		UpdatedAt:     updated,
		Pricing: map[string]map[string]Rate{
			"claude-code": {"opus-4-7": {Input: input, Output: 25, CacheWrite: 6.25, CacheRead: 0.5}},
		},
	}
}

// serveDoc stands up a test server returning the given marshaled body and a 200,
// shrinks the TTL tunables, and returns the URL, a hit counter, and a restore fn.
func serveDoc(t *testing.T, body []byte, status int) (url string, hits *atomic.Int32, restore func()) {
	t.Helper()
	var counter atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		w.WriteHeader(status)
		w.Write(body)
	}))
	origSuccess, origFailure, origTimeout := successTTL, failureTTL, requestTimeout
	successTTL = 1 * time.Hour
	failureTTL = 5 * time.Minute
	requestTimeout = 2 * time.Second
	return srv.URL, &counter, func() {
		srv.Close()
		successTTL, failureTTL, requestTimeout = origSuccess, origFailure, origTimeout
	}
}

func mustJSON(t *testing.T, d Document) []byte {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func opusInput(d Document) float64 {
	return d.Pricing["claude-code"]["opus-4-7"].Input
}

// --------------------------------------------------------------------------
// Embedded artifact
// --------------------------------------------------------------------------

func TestEmbeddedValid(t *testing.T) {
	d := Embedded()
	if err := validate(d); err != nil {
		t.Fatalf("embedded pricing.json failed validation: %v", err)
	}
	if d.SchemaVersion != 0 {
		t.Errorf("embedded schema_version = %d, want 0", d.SchemaVersion)
	}
	if _, ok := d.Pricing["claude-code"]; !ok {
		t.Error("embedded missing claude-code provider")
	}
	if _, ok := d.Pricing["codex"]; !ok {
		t.Error("embedded missing codex provider")
	}
	// Spot-check a representative rate against the known table.
	if got := d.Pricing["claude-code"]["opus-4-7"].Input; got != 5 {
		t.Errorf("embedded opus-4-7 input = %v, want 5", got)
	}
	if got := d.Pricing["codex"]["gpt-5"].CacheRead; got != 0.125 {
		t.Errorf("embedded gpt-5 cacheRead = %v, want 0.125", got)
	}
	if d.UpdatedAt.IsZero() {
		t.Error("embedded updated_at is zero")
	}
}

func TestEmbeddedFamiliesUniqueAcrossProviders(t *testing.T) {
	seen := map[string]string{}
	for provider, families := range Embedded().Pricing {
		for family := range families {
			if other, dup := seen[family]; dup {
				t.Errorf("family %q appears in both %q and %q (flatten would collide)", family, other, provider)
			}
			seen[family] = provider
		}
	}
}

// --------------------------------------------------------------------------
// Effective — freshest-wins + fallback
// --------------------------------------------------------------------------

func TestEffectiveDisabledReturnsEmbeddedNoEgress(t *testing.T) {
	url, hits, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, 0, 99)), http.StatusOK)
	defer restore()
	_ = url // intentionally not used: disabled source must never call out

	s := NewSource(testEmbedded(), "", successTTL) // empty URL ⇒ disabled
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("disabled source returned input %v, want embedded 5", opusInput(got))
	}
	if hits.Load() != 0 {
		t.Errorf("disabled source made %d HTTP calls, want 0", hits.Load())
	}
}

func TestEffectiveRemoteNewerWins(t *testing.T) {
	url, _, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, 0, 99)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 99 {
		t.Errorf("remote-newer returned input %v, want remote 99", opusInput(got))
	}
}

func TestEffectiveRemoteOlderKeepsEmbedded(t *testing.T) {
	url, _, restore := serveDoc(t, mustJSON(t, remoteDoc(olderAt, 0, 99)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("remote-older returned input %v, want embedded 5", opusInput(got))
	}
}

func TestEffectiveRemoteEqualKeepsEmbedded(t *testing.T) {
	url, _, restore := serveDoc(t, mustJSON(t, remoteDoc(embeddedAt, 0, 99)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("remote-equal returned input %v, want embedded 5 (ties to embedded)", opusInput(got))
	}
}

func TestEffectiveUnreachableReturnsEmbedded(t *testing.T) {
	origTimeout := requestTimeout
	requestTimeout = 200 * time.Millisecond
	defer func() { requestTimeout = origTimeout }()

	// Port 1 on localhost: connection refused / unreachable.
	s := NewSource(testEmbedded(), "http://127.0.0.1:1/pricing", successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("unreachable returned input %v, want embedded 5", opusInput(got))
	}
}

func TestEffectiveServerErrorReturnsEmbedded(t *testing.T) {
	url, _, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, 0, 99)), http.StatusInternalServerError)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("5xx returned input %v, want embedded 5", opusInput(got))
	}
}

func TestEffectiveInvalidSchemaTooNewReturnsEmbedded(t *testing.T) {
	url, _, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, maxSchemaVersion+1, 99)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("schema-too-new returned input %v, want embedded 5", opusInput(got))
	}
}

func TestEffectiveInvalidEmptyPricingReturnsEmbedded(t *testing.T) {
	empty := Document{SchemaVersion: 0, UpdatedAt: newerAt, Pricing: map[string]map[string]Rate{}}
	url, _, restore := serveDoc(t, mustJSON(t, empty), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("empty-pricing returned input %v, want embedded 5", opusInput(got))
	}
}

func TestEffectiveInvalidNegativeRateReturnsEmbedded(t *testing.T) {
	url, _, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, 0, -1)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("negative-rate returned input %v, want embedded 5", opusInput(got))
	}
}

func TestEffectiveMalformedJSONReturnsEmbedded(t *testing.T) {
	url, _, restore := serveDoc(t, []byte("{not json"), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	got := s.Effective(context.Background())

	if opusInput(got) != 5 {
		t.Errorf("malformed-json returned input %v, want embedded 5", opusInput(got))
	}
}

// --------------------------------------------------------------------------
// Caching / TTL / last-good
// --------------------------------------------------------------------------

func TestEffectiveCachesWithinTTL(t *testing.T) {
	url, hits, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, 0, 99)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	s.Effective(context.Background())
	s.Effective(context.Background())

	if hits.Load() != 1 {
		t.Errorf("made %d HTTP calls within TTL, want 1 (cached)", hits.Load())
	}
}

func TestEffectiveRefetchesAfterTTLExpiry(t *testing.T) {
	url, hits, restore := serveDoc(t, mustJSON(t, remoteDoc(newerAt, 0, 99)), http.StatusOK)
	defer restore()

	s := NewSource(testEmbedded(), url, successTTL)
	s.Effective(context.Background())
	expireCache(s)
	s.Effective(context.Background())

	if hits.Load() != 2 {
		t.Errorf("made %d HTTP calls after expiry, want 2", hits.Load())
	}
}

func TestEffectiveKeepsLastGoodOnLaterFailure(t *testing.T) {
	var fail atomic.Bool
	var counter atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(mustJSON(t, remoteDoc(newerAt, 0, 99)))
	}))
	defer srv.Close()
	origSuccess, origFailure := successTTL, failureTTL
	successTTL, failureTTL = 1*time.Hour, 5*time.Minute
	defer func() { successTTL, failureTTL = origSuccess, origFailure }()

	s := NewSource(testEmbedded(), srv.URL, successTTL)
	if got := s.Effective(context.Background()); opusInput(got) != 99 {
		t.Fatalf("first fetch input %v, want remote 99", opusInput(got))
	}
	fail.Store(true)
	expireCache(s)
	got := s.Effective(context.Background())
	if opusInput(got) != 99 {
		t.Errorf("after failure input %v, want last-good remote 99 (not embedded)", opusInput(got))
	}
}

// --------------------------------------------------------------------------
// validate — direct unit (covers non-finite which JSON can't carry)
// --------------------------------------------------------------------------

func TestValidateRejectsNonFinite(t *testing.T) {
	d := remoteDoc(newerAt, 0, math.Inf(1))
	if err := validate(d); err == nil {
		t.Error("validate accepted +Inf rate, want error")
	}
}

func TestValidateRejectsZeroUpdatedAt(t *testing.T) {
	d := remoteDoc(time.Time{}, 0, 5)
	if err := validate(d); err == nil {
		t.Error("validate accepted zero updated_at, want error")
	}
}

func TestValidateAcceptsZeroRate(t *testing.T) {
	// Codex cacheWrite is legitimately 0 — must not be rejected.
	d := Document{SchemaVersion: 0, UpdatedAt: newerAt, Pricing: map[string]map[string]Rate{
		"codex": {"gpt-5": {Input: 1.25, Output: 10, CacheWrite: 0, CacheRead: 0.125}},
	}}
	if err := validate(d); err != nil {
		t.Errorf("validate rejected a legitimate zero rate: %v", err)
	}
}

// --------------------------------------------------------------------------
// NewFromEnv (white-box — reads s.url / s.refresh)
// --------------------------------------------------------------------------

func TestNewFromEnvDefaultURL(t *testing.T) {
	// With no env override the constructor must fall back to the canonical URL.
	// (Guarded: only meaningful when the var is absent from the environment.)
	if _, present := os.LookupEnv("PRICING_SOURCE_URL"); present {
		t.Skip("PRICING_SOURCE_URL set in environment")
	}
	s := NewFromEnv(false)
	if s.url != defaultSourceURL {
		t.Errorf("default url = %q, want %q", s.url, defaultSourceURL)
	}
}

func TestNewFromEnvExplicitEmptyDisables(t *testing.T) {
	t.Setenv("PRICING_SOURCE_URL", "")
	s := NewFromEnv(false)
	if s.url != "" {
		t.Errorf("explicit-empty url = %q, want empty (disabled)", s.url)
	}
}

func TestNewFromEnvForceDisabledBlanksURL(t *testing.T) {
	t.Setenv("PRICING_SOURCE_URL", "https://example.test/pricing")
	s := NewFromEnv(true)
	if s.url != "" {
		t.Errorf("forceDisabled url = %q, want empty", s.url)
	}
}

func TestNewFromEnvParsesRefreshInterval(t *testing.T) {
	t.Setenv("PRICING_REFRESH_INTERVAL", "30m")
	s := NewFromEnv(false)
	if s.refresh != 30*time.Minute {
		t.Errorf("refresh = %v, want 30m", s.refresh)
	}
}

func TestNewFromEnvInvalidRefreshFallsBack(t *testing.T) {
	t.Setenv("PRICING_REFRESH_INTERVAL", "not-a-duration")
	s := NewFromEnv(false)
	if s.refresh != successTTL {
		t.Errorf("invalid refresh = %v, want successTTL %v", s.refresh, successTTL)
	}
}

func TestRefreshIntervalReturnsConfigured(t *testing.T) {
	s := NewSource(testEmbedded(), "https://example.test/p", 42*time.Minute)
	if s.RefreshInterval() != 42*time.Minute {
		t.Errorf("RefreshInterval = %v, want 42m", s.RefreshInterval())
	}
}
