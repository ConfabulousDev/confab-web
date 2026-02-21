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

	t.Run("blocks open redirect via double slash", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "//evil.com"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		location := w.Header().Get("Location")
		// Should be sanitized to "/"
		if location != "https://app.example.com/" {
			t.Errorf("Location = %q, want sanitized redirect", location)
		}
	})

	t.Run("blocks open redirect via absolute URL", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/callback", nil)
		r.AddCookie(&http.Cookie{Name: "post_login_redirect", Value: "https://evil.com/steal"})
		w := httptest.NewRecorder()

		handlePostLoginRedirect(w, r, "https://app.example.com", "user@test.com", "", false)

		location := w.Header().Get("Location")
		// Should be sanitized to "/"
		if location != "https://app.example.com/" {
			t.Errorf("Location = %q, want sanitized redirect", location)
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
