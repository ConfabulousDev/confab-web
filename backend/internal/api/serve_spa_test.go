package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
)

const testIndexHTML = "<!doctype html>\n<html>\n  <head>\n    <title>Confab</title>\n  </head>\n  <body><div id=\"root\"></div></body>\n</html>\n"

// newSPAServer builds a Server through NewServer (so env-driven fields are
// read exactly as in production) and returns its serveSPA handler over a
// temp static dir holding testIndexHTML.
func newSPAServer(t *testing.T, demoEmail string) http.Handler {
	t.Helper()
	// Keep NewServer offline: SaaS mode blanks the pricing fetch URL, and the
	// update check is disabled.
	t.Setenv("ENABLE_SAAS_FOOTER", "true")
	t.Setenv("DISABLE_UPDATE_CHECK", "true")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(testIndexHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(nil, nil, &auth.OAuthConfig{DemoIdentityEmail: demoEmail}, nil, BuildInfo{})
	return s.serveSPA(dir)
}

func getBody(t *testing.T, h http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", path, rec.Code)
	}
	b, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Outside demo mode nothing is injected: index.html (root path and SPA
// fallback alike) is served byte-for-byte, so no third-party script can ride
// along in the head.
func TestServeSPAServesIndexUnmodifiedOutsideDemo(t *testing.T) {
	h := newSPAServer(t, "")
	for _, path := range []string{"/", "/index.html", "/sessions/abc"} {
		if body := getBody(t, h, path); body != testIndexHTML {
			t.Errorf("GET %s: index.html must be served unmodified outside demo mode; got %q", path, body)
		}
	}
}

// The demo-banner state hook (CF-483) is the only head injection; it must
// land exactly once, before </head>, with no external script alongside it.
func TestServeSPAStillInjectsDemoBanner(t *testing.T) {
	h := newSPAServer(t, "demo@confabulous.dev")
	body := getBody(t, h, "/")
	want := `<script>window.__DEMO_IDENTITY__ = "demo@confabulous.dev";</script>`
	if n := strings.Count(body, want); n != 1 {
		t.Fatalf("demo banner script must be injected exactly once; found %d in %q", n, body)
	}
	if strings.Index(body, want) > strings.Index(body, "</head>") {
		t.Errorf("demo banner script must be injected before </head>; got %q", body)
	}
	if strings.Contains(body, "<script src=") {
		t.Errorf("demo mode must not inject any external script; got %q", body)
	}
}
