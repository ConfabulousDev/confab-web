package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cspFor(t *testing.T, staticDir string) string {
	t.Helper()
	t.Setenv("STATIC_FILES_DIR", staticDir)
	h := securityHeadersMiddleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header must be set")
	}
	return csp
}

// cspDirective returns the value of one CSP directive (e.g. "font-src"), or "" if absent.
func cspDirective(csp, name string) string {
	for d := range strings.SplitSeq(csp, ";") {
		d = strings.TrimSpace(d)
		if v, ok := strings.CutPrefix(d, name+" "); ok {
			return v
		}
	}
	return ""
}

// The Lobster wordmark font is self-hosted, so no CSP mode may allow Google Fonts.
func TestCSPAllowsNoThirdPartyFonts(t *testing.T) {
	modes := []struct {
		name, staticDir string
	}{
		{"static SPA", "/srv/static"},
		{"API only", ""},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			csp := cspFor(t, m.staticDir)
			for _, origin := range []string{"fonts.googleapis.com", "fonts.gstatic.com"} {
				if strings.Contains(csp, origin) {
					t.Errorf("CSP must not allow %s (Lobster is self-hosted); got %q", origin, csp)
				}
			}
			if got := cspDirective(csp, "font-src"); got != "'self'" {
				t.Errorf("font-src must be exactly 'self'; got %q", got)
			}
			if got := cspDirective(csp, "style-src"); got != "'self' 'unsafe-inline'" {
				t.Errorf("style-src must be exactly 'self' 'unsafe-inline'; got %q", got)
			}
		})
	}
}

// Each mode has exactly one CSP and it allows no third-party origin: the
// only non-'self' sources are inline script/style and data:/https: images.
// A new third-party embed must update this test deliberately.
func TestCSPIsExactlySelfOnlyPerMode(t *testing.T) {
	modes := []struct {
		name, staticDir, want string
	}{
		{"static SPA", "/srv/static",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'"},
		{"API only", "",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'"},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			csp := cspFor(t, m.staticDir)
			if csp != m.want {
				t.Errorf("CSP = %q, want %q", csp, m.want)
			}
			if got := cspDirective(csp, "frame-src"); got != "" {
				t.Errorf("CSP must not set frame-src (no third-party frames); got %q", got)
			}
		})
	}
}
