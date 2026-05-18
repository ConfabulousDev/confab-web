package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// helper: configure the package to point at the test server and shorten TTLs.
// Returns a restore function the caller should defer.
func withServer(t *testing.T, h http.Handler) (baseURL string, restore func()) {
	t.Helper()
	srv := httptest.NewServer(h)

	origBase := githubBaseURL
	origSuccess := successTTL
	origFailure := failureTTL
	origTimeout := requestTimeout

	githubBaseURL = srv.URL
	successTTL = 1 * time.Hour
	failureTTL = 5 * time.Minute
	requestTimeout = 2 * time.Second

	return srv.URL, func() {
		srv.Close()
		githubBaseURL = origBase
		successTTL = origSuccess
		failureTTL = origFailure
		requestTimeout = origTimeout
	}
}

func releasePayload(tagName, htmlURL string, prerelease bool) string {
	b, _ := json.Marshal(map[string]any{
		"tag_name":   tagName,
		"html_url":   htmlURL,
		"prerelease": prerelease,
	})
	return string(b)
}

func TestStatusReturnsDisabledWhenConstructedDisabled(t *testing.T) {
	var calls atomic.Int32
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(releasePayload("v0.5.0", "https://github.com/x/y/releases/tag/v0.5.0", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", true)
	got := c.Status(context.Background())

	if !got.UpdateCheckDisabled {
		t.Errorf("UpdateCheckDisabled = false, want true")
	}
	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false when disabled")
	}
	if got.Current != "v0.4.1" {
		t.Errorf("Current = %q, want %q", got.Current, "v0.4.1")
	}
	if got.Latest != "" {
		t.Errorf("Latest = %q, want empty when disabled", got.Latest)
	}
	if calls.Load() != 0 {
		t.Errorf("GitHub called %d times, want 0 when disabled", calls.Load())
	}
}

func TestStatusFetchesAndDetectsUpdate(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(releasePayload("v0.5.0", "https://github.com/x/y/releases/tag/v0.5.0", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	got := c.Status(context.Background())

	if !got.UpdateAvailable {
		t.Errorf("UpdateAvailable = false, want true (v0.4.1 < v0.5.0)")
	}
	if got.Latest != "v0.5.0" {
		t.Errorf("Latest = %q, want %q", got.Latest, "v0.5.0")
	}
	if got.LatestURL != "https://github.com/x/y/releases/tag/v0.5.0" {
		t.Errorf("LatestURL = %q, want the GitHub html_url verbatim", got.LatestURL)
	}
	if got.UpdateCheckFailed {
		t.Errorf("UpdateCheckFailed = true, want false on success")
	}
	if got.UpdateCheckDisabled {
		t.Errorf("UpdateCheckDisabled = true, want false when enabled")
	}
}

func TestStatusReportsNoUpdateWhenCurrentIsLatest(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.5.0", false)
	got := c.Status(context.Background())

	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false when current == latest")
	}
}

func TestStatusReportsNoUpdateWhenCurrentIsAhead(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(releasePayload("v0.4.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.5.0", false)
	got := c.Status(context.Background())

	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false when current is ahead of latest")
	}
}

func TestStatusFiltersPrereleases(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(releasePayload("v0.5.0-rc.1", "https://example.test/r", true)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	got := c.Status(context.Background())

	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false when GitHub release is a prerelease")
	}
}

func TestStatusCachesSuccessWithinTTL(t *testing.T) {
	var calls atomic.Int32
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	c.Status(context.Background())
	c.Status(context.Background())
	c.Status(context.Background())

	if calls.Load() != 1 {
		t.Errorf("GitHub called %d times across 3 Status calls, want 1 (TTL caching)", calls.Load())
	}
}

func TestStatusRefetchesAfterSuccessTTL(t *testing.T) {
	var calls atomic.Int32
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	c.Status(context.Background())

	// Force the cache to look stale.
	expireCache(c)

	c.Status(context.Background())

	if calls.Load() != 2 {
		t.Errorf("GitHub called %d times after forced cache expiry, want 2", calls.Load())
	}
}

func TestStatusReportsFailedOn5xx(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	got := c.Status(context.Background())

	if !got.UpdateCheckFailed {
		t.Errorf("UpdateCheckFailed = false, want true on 5xx")
	}
	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false on failure")
	}
	if got.Latest != "" {
		t.Errorf("Latest = %q, want empty on failure", got.Latest)
	}
	if got.Current != "v0.4.1" {
		t.Errorf("Current should still be reported on failure, got %q", got.Current)
	}
}

func TestStatusReportsFailedOnInvalidJSON(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json at all"))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	got := c.Status(context.Background())

	if !got.UpdateCheckFailed {
		t.Errorf("UpdateCheckFailed = false, want true on malformed JSON")
	}
}

func TestStatusCachesFailureWithinFailureTTL(t *testing.T) {
	var calls atomic.Int32
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	c.Status(context.Background())
	c.Status(context.Background())
	c.Status(context.Background())

	if calls.Load() != 1 {
		t.Errorf("GitHub called %d times across 3 Status calls after failure, want 1 (failure cooldown)", calls.Load())
	}
}

func TestStatusForcesUpdateAvailableWhenCurrentIsEmpty(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("", false)
	got := c.Status(context.Background())

	if !got.UpdateAvailable {
		t.Errorf("UpdateAvailable = false, want true when current is empty (dev bias)")
	}
	if got.Latest != "v0.5.0" {
		t.Errorf("Latest = %q, want %q (should still surface real GitHub data in dev)", got.Latest, "v0.5.0")
	}
	if got.UpdateCheckDisabled {
		t.Errorf("UpdateCheckDisabled = true, want false in dev bias mode")
	}
}

func TestStatusReportsFailedWhenCurrentEmptyAndGithubFails(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer restore()

	c := NewChecker("", false)
	got := c.Status(context.Background())

	if !got.UpdateCheckFailed {
		t.Errorf("UpdateCheckFailed = false, want true on dev fetch failure")
	}
	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false when fetch failed (no link to send the user to)")
	}
}

func TestStatusSendsExpectedHeaders(t *testing.T) {
	var gotUA, gotAccept string
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	c.Status(context.Background())

	wantUA := "confab-backend/v0.4.1"
	if gotUA != wantUA {
		t.Errorf("User-Agent = %q, want %q", gotUA, wantUA)
	}
	if !strings.Contains(gotAccept, "application/vnd.github+json") {
		t.Errorf("Accept = %q, want it to include application/vnd.github+json", gotAccept)
	}
}

func TestStatusHitsReleasesLatestEndpoint(t *testing.T) {
	var gotPath string
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	c.Status(context.Background())

	wantPath := "/repos/ConfabulousDev/confab-web/releases/latest"
	if gotPath != wantPath {
		t.Errorf("GitHub path = %q, want %q", gotPath, wantPath)
	}
}

func TestStatusRespectsContextCancellation(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block long enough to outlive a cancelled context.
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(releasePayload("v0.5.0", "https://example.test/r", false)))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	got := c.Status(ctx)
	if !got.UpdateCheckFailed {
		t.Errorf("UpdateCheckFailed = false, want true when fetch is cancelled/times out")
	}
}

func TestStatusHandlesEmptyTagFromGithub(t *testing.T) {
	_, restore := withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name": "", "html_url": "", "prerelease": false}`))
	}))
	defer restore()

	c := NewChecker("v0.4.1", false)
	got := c.Status(context.Background())

	if got.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false when GitHub returned an empty tag")
	}
	if got.UpdateCheckFailed {
		// Empty tag is treated as malformed; either failed=true or available=false is acceptable
		// but we should NOT claim update_available=true.
		fmt.Printf("(empty tag treated as failure, also acceptable)\n")
	}
}
