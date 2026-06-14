package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrossOriginAllowed(t *testing.T) {
	// Trusted cross-origin allowlist (e.g. a separate frontend origin).
	trusted := map[string]struct{}{"https://app.example.com": {}}

	cases := []struct {
		name      string
		method    string
		host      string
		secFetch  string
		origin    string
		wantAllow bool
	}{
		// Sec-Fetch-Site is the primary signal.
		{"same-origin nav", "GET", "confab.example.com", "same-origin", "", true},
		{"direct nav (none)", "GET", "confab.example.com", "none", "", true},
		{"cross-site POST", "POST", "confab.example.com", "cross-site", "https://evil.example.com", false},
		{"same-site POST", "POST", "confab.example.com", "same-site", "", false},
		{"cross-site GET (state-changing GET must still be blocked)", "GET", "confab.example.com", "cross-site", "", false},

		// Sec-Fetch-Site absent → Origin fallback.
		{"no sec-fetch, same-origin Origin", "POST", "confab.example.com", "", "https://confab.example.com", true},
		{"no sec-fetch, trusted Origin", "POST", "confab.example.com", "", "https://app.example.com", true},
		{"no sec-fetch, untrusted Origin", "POST", "confab.example.com", "", "https://evil.example.com", false},
		{"no sec-fetch, no Origin (fail closed)", "POST", "confab.example.com", "", "", false},
		{"no sec-fetch, no Origin on GET (fail closed)", "GET", "confab.example.com", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "https://"+tc.host+"/auth/cli/authorize", nil)
			req.Host = tc.host
			if tc.secFetch != "" {
				req.Header.Set("Sec-Fetch-Site", tc.secFetch)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if got := crossOriginAllowed(req, trusted); got != tc.wantAllow {
				t.Errorf("crossOriginAllowed = %v, want %v", got, tc.wantAllow)
			}
		})
	}
}

// TestCrossOriginGuard_RejectsAndPasses asserts the middleware returns 403 on a
// blocked request and calls through on an allowed one.
func TestCrossOriginGuard_RejectsAndPasses(t *testing.T) {
	called := false
	next := func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) }
	guard := crossOriginGuard([]string{"https://app.example.com"}, next)

	// Cross-site → 403, next not called.
	called = false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://confab.example.com/auth/device/verify", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	guard(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site: status = %d, want 403", rec.Code)
	}
	if called {
		t.Error("cross-site: next handler must not be called")
	}

	// Same-origin → passes through.
	called = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "https://confab.example.com/auth/device/verify", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	guard(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Errorf("same-origin: status = %d, called = %v, want 200/true", rec.Code, called)
	}
}
