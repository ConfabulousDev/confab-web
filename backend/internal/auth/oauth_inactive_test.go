package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestRedirectInactiveUser covers the shared redirect used by the three OAuth
// callbacks when FindOrCreateUserByOAuth resolves to a deactivated account
// (w8tz). Rejecting before CreateWebSession is what breaks the login loop: no
// session is ever minted, so the user can't bounce app→401→login→app again.
//
// The copy must stay generic ("not active / contact support") and must NOT
// confirm that the account specifically is deactivated (the ticket says don't
// reveal too much) — so the error code is account_inactive but the visible
// description is the same regardless of whether the account exists.
func TestRedirectInactiveUser(t *testing.T) {
	req := httptest.NewRequest("GET", "/auth/google/callback", nil)
	rec := httptest.NewRecorder()

	redirectInactiveUser(rec, req, "http://frontend.test")

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("Location header not set")
	}

	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location is not a valid URL: %v", err)
	}
	if parsed.Path != "/login" {
		t.Errorf("redirect path = %q, want /login", parsed.Path)
	}
	if got := parsed.Query().Get("error"); got != "account_inactive" {
		t.Errorf("error = %q, want account_inactive", got)
	}

	desc := parsed.Query().Get("error_description")
	if desc == "" {
		t.Error("error_description must be present so the login page shows a friendly message")
	}
	// Must NOT confirm deactivation specifically (don't reveal too much).
	for _, leak := range []string{"deactivat", "disabled", "inactive", "suspend"} {
		if strings.Contains(strings.ToLower(desc), leak) {
			t.Errorf("error_description %q leaks account state (contains %q)", desc, leak)
		}
	}
	// Must point the user at support.
	if !strings.Contains(strings.ToLower(desc), "support") {
		t.Errorf("error_description %q should direct the user to contact support", desc)
	}

	// No session cookie may be set on the rejection path.
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			t.Errorf("inactive rejection must not set a session cookie, got %q", c.Value)
		}
	}
}
