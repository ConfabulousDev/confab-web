package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cspFor(t *testing.T, staticDir, termly string) string {
	t.Helper()
	t.Setenv("STATIC_FILES_DIR", staticDir)
	t.Setenv("ENABLE_SAAS_TERMLY", termly)
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
		name, staticDir, termly string
	}{
		{"static SPA", "/srv/static", "false"},
		{"static SPA with Termly", "/srv/static", "true"},
		{"API only", "", "false"},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			csp := cspFor(t, m.staticDir, m.termly)
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

// Removing Google Fonts must not drop the Termly origins; jks2 removes those separately.
func TestCSPTermlyBranchKeepsTermlyOrigins(t *testing.T) {
	termlyCSP := cspFor(t, "/srv/static", "true")
	if !strings.Contains(cspDirective(termlyCSP, "script-src"), "https://app.termly.io") {
		t.Errorf("Termly CSP must still allow app.termly.io scripts; got %q", termlyCSP)
	}
	plainCSP := cspFor(t, "/srv/static", "false")
	if strings.Contains(plainCSP, "termly") {
		t.Errorf("non-Termly CSP must not mention termly; got %q", plainCSP)
	}
}
