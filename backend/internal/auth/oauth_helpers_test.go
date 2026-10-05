package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppendEmailMismatchParams(t *testing.T) {
	t.Run("appends with ? when no existing query", func(t *testing.T) {
		result := appendEmailMismatchParams("https://example.com", "expected@test.com", "actual@test.com")
		want := "https://example.com?email_mismatch=1&expected=expected%40test.com&actual=actual%40test.com"
		if result != want {
			t.Errorf("got %q, want %q", result, want)
		}
	})

	t.Run("appends with & when query params exist", func(t *testing.T) {
		result := appendEmailMismatchParams("https://example.com?foo=bar", "expected@test.com", "actual@test.com")
		want := "https://example.com?foo=bar&email_mismatch=1&expected=expected%40test.com&actual=actual%40test.com"
		if result != want {
			t.Errorf("got %q, want %q", result, want)
		}
	})

	t.Run("escapes special characters in emails", func(t *testing.T) {
		result := appendEmailMismatchParams("https://example.com", "user+tag@test.com", "other@test.com")
		want := "https://example.com?email_mismatch=1&expected=user%2Btag%40test.com&actual=other%40test.com"
		if result != want {
			t.Errorf("got %q, want %q", result, want)
		}
	})
}

func TestCheckExpectedEmailMismatch(t *testing.T) {
	t.Run("returns false when no cookie", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		w := httptest.NewRecorder()

		expected, mismatch := checkExpectedEmailMismatch(w, r, "user@test.com", "github")
		if expected != "" {
			t.Errorf("expected empty string, got %q", expected)
		}
		if mismatch {
			t.Error("expected no mismatch when cookie absent")
		}
	})

	t.Run("returns false when cookie is empty", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "expected_email", Value: ""})
		w := httptest.NewRecorder()

		expected, mismatch := checkExpectedEmailMismatch(w, r, "user@test.com", "github")
		if expected != "" {
			t.Errorf("expected empty string, got %q", expected)
		}
		if mismatch {
			t.Error("expected no mismatch when cookie is empty")
		}
	})

	t.Run("returns false when emails match", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "expected_email", Value: "user@test.com"})
		w := httptest.NewRecorder()

		expected, mismatch := checkExpectedEmailMismatch(w, r, "user@test.com", "github")
		if expected != "user@test.com" {
			t.Errorf("expected %q, got %q", "user@test.com", expected)
		}
		if mismatch {
			t.Error("expected no mismatch when emails match")
		}
		// Cookie should be cleared
		assertCookieCleared(t, w, "expected_email")
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "expected_email", Value: "User@Test.COM"})
		w := httptest.NewRecorder()

		expected, mismatch := checkExpectedEmailMismatch(w, r, "user@test.com", "google")
		if expected != "User@Test.COM" {
			t.Errorf("expected %q, got %q", "User@Test.COM", expected)
		}
		if mismatch {
			t.Error("expected no mismatch for case-insensitive match")
		}
	})

	t.Run("returns true when emails differ", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "expected_email", Value: "expected@test.com"})
		w := httptest.NewRecorder()

		expected, mismatch := checkExpectedEmailMismatch(w, r, "actual@test.com", "oidc")
		if expected != "expected@test.com" {
			t.Errorf("expected %q, got %q", "expected@test.com", expected)
		}
		if !mismatch {
			t.Error("expected mismatch when emails differ")
		}
		// Cookie should still be cleared
		assertCookieCleared(t, w, "expected_email")
	})
}

func TestHandlePostLoginRedirect(t *testing.T) {
	t.Run("redirects to CLI when cli_redirect cookie set", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "cli_redirect", Value: "/auth/cli/authorize?callback=http://localhost:8080"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("status = %d, want %d", w.Code, http.StatusTemporaryRedirect)
		}
		location := w.Header().Get("Location")
		if location != "/auth/cli/authorize?callback=http://localhost:8080" {
			t.Errorf("Location = %q, want CLI redirect", location)
		}
	})

	t.Run("redirects to post_login_redirect frontend path", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "/sessions/abc123"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("status = %d, want %d", w.Code, http.StatusTemporaryRedirect)
		}
		location := w.Header().Get("Location")
		if location != "https://app.example.com/sessions/abc123" {
			t.Errorf("Location = %q, want frontend URL + path", location)
		}
	})

	t.Run("backend paths not prepended with frontend URL", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "/auth/device?code=ABCD-1234"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		location := w.Header().Get("Location")
		if location != "/auth/device?code=ABCD-1234" {
			t.Errorf("Location = %q, want backend path without frontend prefix", location)
		}
	})

	// An invalid post_login_redirect falls back to the default frontend URL
	// (and the cookie is still cleared so it cannot be replayed). Backslash and
	// control-char cases live in TestResolvePostLoginRedirect: net/http strips
	// those bytes from cookie values before they reach the handler.
	for _, bad := range []string{"//evil.com", "https://evil.com/steal"} {
		t.Run("blocks open redirect "+bad, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/callback", nil)
			r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: bad})
			w := httptest.NewRecorder()

			handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

			if location := w.Header().Get("Location"); location != "https://app.example.com" {
				t.Errorf("Location = %q, want default frontend URL", location)
			}
			assertCookieCleared(t, w, "post_login_redirect")
		})
	}

	t.Run("appends email mismatch params to the default after a blocked redirect", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "//evil.com"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "actual@test.com", "expected@test.com", true)

		want := "https://app.example.com?email_mismatch=1&expected=expected%40test.com&actual=actual%40test.com"
		if location := w.Header().Get("Location"); location != want {
			t.Errorf("Location = %q, want %q", location, want)
		}
	})

	for _, path := range []string{"/authors", "/device"} {
		t.Run("frontend-prefixes "+path+" (only /auth, /auth/... and /auth?... are backend paths)", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/callback", nil)
			r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: path})
			w := httptest.NewRecorder()

			handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

			if location := w.Header().Get("Location"); location != "https://app.example.com"+path {
				t.Errorf("Location = %q, want frontend URL + %s", location, path)
			}
		})
	}

	t.Run("CLI redirect wins over post_login_redirect", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "cli_redirect", Value: "/auth/cli/authorize?callback=x"})
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "/sessions/abc"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		if location := w.Header().Get("Location"); location != "/auth/cli/authorize?callback=x" {
			t.Errorf("Location = %q, want the CLI redirect", location)
		}
	})

	t.Run("falls back to frontend URL", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("status = %d, want %d", w.Code, http.StatusTemporaryRedirect)
		}
		location := w.Header().Get("Location")
		if location != "https://app.example.com" {
			t.Errorf("Location = %q, want frontend URL", location)
		}
	})

	t.Run("appends email mismatch params to frontend redirect", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "actual@test.com", "expected@test.com", true)

		location := w.Header().Get("Location")
		want := "https://app.example.com?email_mismatch=1&expected=expected%40test.com&actual=actual%40test.com"
		if location != want {
			t.Errorf("Location = %q, want %q", location, want)
		}
	})

	t.Run("appends email mismatch params to post_login_redirect", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "/sessions/abc"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "actual@test.com", "expected@test.com", true)

		location := w.Header().Get("Location")
		want := "https://app.example.com/sessions/abc?email_mismatch=1&expected=expected%40test.com&actual=actual%40test.com"
		if location != want {
			t.Errorf("Location = %q, want %q", location, want)
		}
	})
}

// assertCookieCleared checks that a Set-Cookie header clears the named cookie.
func assertCookieCleared(t *testing.T, w *httptest.ResponseRecorder, name string) {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == name && cookie.MaxAge == -1 {
			return
		}
	}
	t.Errorf("expected cookie %q to be cleared (MaxAge=-1)", name)
}

func TestResolvePostLoginRedirect(t *testing.T) {
	const frontend = "https://app.example.com"
	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{"empty is no target", "", "", false},
		{"absolute URL is rejected", "http://evil.com", "", false},
		{"protocol-relative is rejected", "//evil.com", "", false},
		{"leading backslash is rejected", "/\\evil.com", "", false},
		{"backslash anywhere is rejected", "/a\\b", "", false},
		{"NUL control char is rejected", "/x\x00", "", false},
		{"newline control char is rejected", "/x\nSet-Cookie:a=b", "", false},
		{"tab control char is rejected", "/\t/evil.com", "", false},
		{"DEL control char is rejected", "/x\x7f", "", false},
		{"no leading slash is rejected", "sessions/9", "", false},
		{"bare /auth is a backend path", "/auth", "/auth", true},
		{"/auth/... is a backend path", "/auth/device?code=ABCD-2345", "/auth/device?code=ABCD-2345", true},
		{"/auth?query is a backend path", "/auth?x=1", "/auth?x=1", true},
		{"/authors is a frontend path", "/authors", frontend + "/authors", true},
		{"/device is a frontend path", "/device", frontend + "/device", true},
		{"frontend path is prefixed", "/sessions/9", frontend + "/sessions/9", true},
		{"frontend path with query is prefixed", "/login?redirect=%2Fsessions", frontend + "/login?redirect=%2Fsessions", true},
		{"root is prefixed", "/", frontend + "/", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolvePostLoginRedirect(tt.raw, frontend)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("resolvePostLoginRedirect(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
