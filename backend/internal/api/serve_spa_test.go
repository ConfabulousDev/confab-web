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
	return newSPAServerWithIndex(t, demoEmail, testIndexHTML)
}

// newSPAServerWithIndex is newSPAServer over a caller-supplied index.html.
func newSPAServerWithIndex(t *testing.T, demoEmail, indexHTML string) http.Handler {
	t.Helper()
	// Keep NewServer offline: SaaS mode blanks the pricing fetch URL, and the
	// update check is disabled.
	t.Setenv("ENABLE_SAAS_FOOTER", "true")
	t.Setenv("DISABLE_UPDATE_CHECK", "true")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexHTML), 0o644); err != nil {
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

// The demo-banner state hook (CF-483) is the only script injected into the
// head; it must land exactly once, before </head>, with no external script
// alongside it.
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

const testOGIndexHTML = "<!doctype html>\n<html>\n  <head>\n" +
	"    <meta property=\"og:image\" content=\"/og-image.png\" />\n" +
	"    <meta name=\"twitter:image\" content=\"/og-image.png\" />\n" +
	"  </head>\n  <body><div id=\"root\"></div></body>\n</html>\n"

// wantAbsoluteOGImage is the rewritten og:image / twitter:image attribute for
// FRONTEND_URL https://x.example (with or without a trailing slash).
const wantAbsoluteOGImage = `content="https://x.example/og-image.png"`

// Link-preview crawlers need an absolute og:image / twitter:image URL, so
// serveSPA rewrites the relative /og-image.png to FRONTEND_URL (trailing
// slash trimmed) + /og-image.png at startup.
func TestServeSPAInjectsAbsoluteOGImageURL(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://x.example/")
	h := newSPAServerWithIndex(t, "", testOGIndexHTML)
	for _, path := range []string{"/", "/sessions/abc"} {
		body := getBody(t, h, path)
		if n := strings.Count(body, wantAbsoluteOGImage); n != 2 {
			t.Errorf("GET %s: og:image and twitter:image must both use the absolute URL; found %d in %q", path, n, body)
		}
		if strings.Contains(body, `content="/og-image.png"`) {
			t.Errorf("GET %s: no relative og-image URL may remain; got %q", path, body)
		}
		if strings.Contains(body, "x.example//og-image.png") {
			t.Errorf("GET %s: trailing slash on FRONTEND_URL must not double up; got %q", path, body)
		}
	}
}

func TestServeSPAOGImageURLWithoutTrailingSlash(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://x.example")
	body := getBody(t, newSPAServerWithIndex(t, "", testOGIndexHTML), "/")
	if n := strings.Count(body, wantAbsoluteOGImage); n != 2 {
		t.Errorf("og image URLs must be absolute; found %d in %q", n, body)
	}
}

// Without FRONTEND_URL there is no origin to build from, so the relative
// path is left alone rather than rewritten to a bare "/og-image.png" host.
func TestServeSPALeavesOGImageRelativeWithoutFrontendURL(t *testing.T) {
	t.Setenv("FRONTEND_URL", "")
	if body := getBody(t, newSPAServerWithIndex(t, "", testOGIndexHTML), "/"); body != testOGIndexHTML {
		t.Errorf("index.html must be unchanged without FRONTEND_URL; got %q", body)
	}
}

// The OG rewrite composes with the demo banner: both apply, and the banner is
// still injected exactly once before </head>.
func TestServeSPAOGImageInjectionKeepsDemoBanner(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://x.example/")
	body := getBody(t, newSPAServerWithIndex(t, "demo@confabulous.dev", testOGIndexHTML), "/")
	want := `<script>window.__DEMO_IDENTITY__ = "demo@confabulous.dev";</script>`
	if n := strings.Count(body, want); n != 1 {
		t.Fatalf("demo banner script must be injected exactly once; found %d in %q", n, body)
	}
	if strings.Index(body, want) > strings.Index(body, "</head>") {
		t.Errorf("demo banner script must be injected before </head>; got %q", body)
	}
	if n := strings.Count(body, wantAbsoluteOGImage); n != 2 {
		t.Errorf("og image URLs must be absolute in demo mode too; found %d in %q", n, body)
	}
}

// FRONTEND_URL is operator config, but it lands inside an HTML attribute, so
// it is attribute-escaped: a stray quote cannot break out of content="...".
func TestServeSPAEscapesFrontendURLInOGImage(t *testing.T) {
	t.Setenv("FRONTEND_URL", `https://x.example/"><script>`)
	body := getBody(t, newSPAServerWithIndex(t, "", testOGIndexHTML), "/")
	if strings.Contains(body, `"><script>`) {
		t.Fatalf("FRONTEND_URL must be HTML-escaped in the og image URL; got %q", body)
	}
	if n := strings.Count(body, `content="https://x.example/&#34;&gt;&lt;script&gt;/og-image.png"`); n != 2 {
		t.Errorf("escaped og image URL must appear twice; found %d in %q", n, body)
	}
}
