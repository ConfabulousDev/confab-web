package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// noRedirectClient returns a client that does not follow redirects, so we can
// observe the guard's 403 vs. the handler's login redirect.
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// TestCrossOriginGuard_BlocksStateChangingAuthRoutes is the wire-level check for
// 56mw: a cross-site request to /auth/cli/authorize (state-changing GET) or
// /auth/device/verify (POST) is rejected with 403 before reaching the handler,
// while a same-origin request passes the guard (and falls through to the
// handler's own auth check — here a login redirect, NOT a 403).
func TestCrossOriginGuard_BlocksStateChangingAuthRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	ts := setupDeviceCodeTestServer(t, env)
	client := noRedirectClient()

	type tc struct {
		name        string
		method      string
		path        string
		body        string
		secFetch    string
		wantBlocked bool
	}
	cases := []tc{
		{"cli/authorize cross-site GET blocked", "GET", "/auth/cli/authorize", "", "cross-site", true},
		{"cli/authorize same-origin GET passes guard", "GET", "/auth/cli/authorize", "", "same-origin", false},
		{"device/verify cross-site POST blocked", "POST", "/auth/device/verify", "code=WXYZ-1234", "cross-site", true},
		{"device/verify same-origin POST passes guard", "POST", "/auth/device/verify", "code=WXYZ-1234", "same-origin", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := http.NewRequest(c.method, ts.URL+c.path, strings.NewReader(c.body))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			if c.method == "POST" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			req.Header.Set("Sec-Fetch-Site", c.secFetch)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			defer resp.Body.Close()

			if c.wantBlocked {
				if resp.StatusCode != http.StatusForbidden {
					t.Errorf("expected 403 (guard block), got %d", resp.StatusCode)
				}
			} else {
				// Guard passed → handler ran. With no session it redirects to
				// login; the one thing it must NOT be is the guard's 403.
				if resp.StatusCode == http.StatusForbidden {
					t.Errorf("same-origin request was blocked (403); guard should have passed it through")
				}
			}
		})
	}
}
