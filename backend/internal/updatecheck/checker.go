// Package updatecheck reports whether the running backend build is behind the
// latest GitHub release of confab-web. Powers the "Update available" badge on
// /api/v1/auth/config.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/logger"
	"golang.org/x/mod/semver"
)

// Tunables are vars (not consts) so tests can shrink them.
var (
	successTTL     = 6 * time.Hour
	failureTTL     = 15 * time.Minute
	requestTimeout = 3 * time.Second
	githubBaseURL  = "https://api.github.com"
)

const githubRepo = "ConfabulousDev/confab-web"

// Status is the snapshot serialized as the `version` object on
// /api/v1/auth/config.
type Status struct {
	Current             string `json:"current"`
	Latest              string `json:"latest,omitempty"`
	LatestURL           string `json:"latest_url,omitempty"`
	UpdateAvailable     bool   `json:"update_available"`
	UpdateCheckDisabled bool   `json:"update_check_disabled"`
	UpdateCheckFailed   bool   `json:"update_check_failed"`
}

// Checker manages the lazy GitHub fetch + TTL cache.
type Checker struct {
	version  string
	disabled bool
	client   *http.Client

	mu        sync.Mutex
	cached    Status
	fetchedAt time.Time
	cacheTTL  time.Duration // successTTL on hit, failureTTL on miss
}

// NewChecker binds a Checker to the running version. If disabled is true the
// checker never contacts GitHub.
func NewChecker(version string, disabled bool) *Checker {
	return &Checker{
		version:  version,
		disabled: disabled,
		client:   &http.Client{Timeout: requestTimeout},
	}
}

// Status returns the cached update status, refreshing from GitHub if the
// cache is stale and the checker isn't disabled.
func (c *Checker) Status(ctx context.Context) Status {
	if c.disabled {
		return Status{Current: c.version, UpdateCheckDisabled: true}
	}

	c.mu.Lock()
	if !c.fetchedAt.IsZero() && time.Since(c.fetchedAt) < c.cacheTTL {
		cached := c.cached
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	// Fetch outside the lock so concurrent callers don't queue on the network.
	// Two concurrent stale-cache requests may both fetch; accepted per design.
	tag, htmlURL, err := c.fetch(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		logger.Warn("github release check failed", "error", err)
		c.cached = Status{Current: c.version, UpdateCheckFailed: true}
		c.fetchedAt = time.Now()
		c.cacheTTL = failureTTL
		return c.cached
	}

	c.cached = Status{
		Current:         c.version,
		Latest:          tag,
		LatestURL:       htmlURL,
		UpdateAvailable: shouldBadge(c.version, tag),
	}
	c.fetchedAt = time.Now()
	c.cacheTTL = successTTL
	return c.cached
}

// shouldBadge decides whether to surface the "update available" badge.
//
// Dev bias: when current is empty (local `go run` build) we force the badge on
// so devs can see it without faking a version.
func shouldBadge(current, latest string) bool {
	if latest == "" {
		return false
	}
	if current == "" {
		return true
	}
	if !semver.IsValid(current) || !semver.IsValid(latest) {
		return false
	}
	return semver.Compare(latest, current) > 0
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
}

func (c *Checker) fetch(ctx context.Context) (tag, htmlURL string, err error) {
	fetchCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubBaseURL, githubRepo)
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("confab-backend/%s", versionOrUnknown(c.version)))
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("github returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", err
	}
	if rel.Prerelease {
		return "", "", errors.New("latest release is a prerelease; ignoring")
	}
	if rel.TagName == "" {
		return "", "", errors.New("github response missing tag_name")
	}
	return rel.TagName, rel.HTMLURL, nil
}

func versionOrUnknown(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

// expireCache is a test-only seam that backdates the cached fetch time so
// the next Status() call refetches.
func expireCache(c *Checker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetchedAt = time.Time{}
}
