package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestRedirectUserIneligible maps each checkUserEligibility outcome to its
// login-page error code. The server_error branch is not reachable through the
// callbacks without sabotaging the DB, so the mapping is asserted directly.
func TestRedirectUserIneligible(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
		wantDesc string
	}{
		{"domain not permitted", errEmailDomainNotPermitted, "access_denied", "Your email domain is not permitted. Contact your administrator."},
		{"wrapped user cap", fmt.Errorf("check: %w", errUserCapReached), "access_denied", "This application has reached its user limit. Please contact the administrator."},
		{"eligibility check failure", errors.New("connection refused"), "server_error", "An error occurred. Please try again later."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			redirectUserIneligible(rec, httptest.NewRequest(http.MethodGet, "/cb", nil), "http://frontend.test", "github", "a@example.com", c.err)

			if rec.Code != http.StatusTemporaryRedirect {
				t.Fatalf("status = %d, want 307", rec.Code)
			}
			raw := rec.Header().Get("Location")
			loc, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("bad Location %q: %v", raw, err)
			}
			if got := loc.Scheme + "://" + loc.Host + loc.Path; got != "http://frontend.test/login" {
				t.Errorf("redirect = %q, want http://frontend.test/login", got)
			}
			if got := loc.Query().Get("error"); got != c.wantCode {
				t.Errorf("error = %q, want %q", got, c.wantCode)
			}
			if got := loc.Query().Get("error_description"); got != c.wantDesc {
				t.Errorf("error_description = %q, want %q", got, c.wantDesc)
			}
			// Internal error text must never reach the login page.
			if strings.Contains(raw, url.QueryEscape(c.err.Error())) {
				t.Errorf("internal error text leaked into redirect: %q", raw)
			}
		})
	}
}

// TestValidateOAuthCallback covers the shared state+PKCE+code validation block
// extracted from the three OAuth callbacks (e7py). It must reproduce the exact
// behavior each callback had inline: reject bad state, reject missing/empty
// verifier, reject missing code, and clear both cookies on the happy path.
func TestValidateOAuthCallback(t *testing.T) {
	t.Run("rejects missing state cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cb?state=s&code=c", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_verifier", Value: "v"})
		rec := httptest.NewRecorder()

		_, _, err := validateOAuthCallback(rec, req)
		if err == nil {
			t.Fatal("expected error for missing state cookie")
		}
	})

	t.Run("rejects state mismatch", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cb?state=query_state&code=c", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "cookie_state"})
		req.AddCookie(&http.Cookie{Name: "oauth_verifier", Value: "v"})
		rec := httptest.NewRecorder()

		_, _, err := validateOAuthCallback(rec, req)
		if err == nil {
			t.Fatal("expected error for state mismatch")
		}
	})

	t.Run("rejects missing verifier cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cb?state=s&code=c", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "s"})
		// no oauth_verifier
		rec := httptest.NewRecorder()

		_, _, err := validateOAuthCallback(rec, req)
		if err == nil {
			t.Fatal("expected error for missing verifier cookie")
		}
	})

	t.Run("rejects empty verifier cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cb?state=s&code=c", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "s"})
		req.AddCookie(&http.Cookie{Name: "oauth_verifier", Value: ""})
		rec := httptest.NewRecorder()

		_, _, err := validateOAuthCallback(rec, req)
		if err == nil {
			t.Fatal("expected error for empty verifier cookie")
		}
	})

	t.Run("rejects missing code", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cb?state=s", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "s"})
		req.AddCookie(&http.Cookie{Name: "oauth_verifier", Value: "v"})
		rec := httptest.NewRecorder()

		_, _, err := validateOAuthCallback(rec, req)
		if err == nil {
			t.Fatal("expected error for missing code")
		}
	})

	t.Run("happy path returns code+verifier and clears both cookies", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cb?state=s&code=auth_code", nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "s"})
		req.AddCookie(&http.Cookie{Name: "oauth_verifier", Value: "the_verifier"})
		rec := httptest.NewRecorder()

		code, verifier, err := validateOAuthCallback(rec, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != "auth_code" {
			t.Errorf("code = %q, want %q", code, "auth_code")
		}
		if verifier != "the_verifier" {
			t.Errorf("verifier = %q, want %q", verifier, "the_verifier")
		}

		// Both cookies must be cleared (MaxAge=-1) for defense-in-depth.
		cleared := map[string]bool{}
		for _, c := range rec.Result().Cookies() {
			if c.MaxAge == -1 {
				cleared[c.Name] = true
			}
		}
		if !cleared["oauth_state"] {
			t.Error("oauth_state cookie was not cleared")
		}
		if !cleared["oauth_verifier"] {
			t.Error("oauth_verifier cookie was not cleared")
		}
	})
}

// TestCheckUserEligibility covers the shared email-domain + user-cap block
// extracted from the three OAuth callbacks (e7py). With a nil database the
// disallowed-domain check still rejects before any DB access; the eligible and
// over-cap paths require a real DB and are exercised by the callback integration
// tests, so here we only assert the domain gate and that a disallowed domain
// short-circuits without touching the DB.
func TestCheckUserEligibility(t *testing.T) {
	t.Run("rejects disallowed domain with errEmailDomainNotPermitted", func(t *testing.T) {
		// nil db proves the domain check runs first and short-circuits.
		err := checkUserEligibility(context.Background(), nil, "user@evil.com", []string{"good.com"})
		if !errors.Is(err, errEmailDomainNotPermitted) {
			t.Fatalf("err = %v, want errEmailDomainNotPermitted", err)
		}
	})

	t.Run("rejects invalid email under domain restriction before DB access", func(t *testing.T) {
		// With a domain allow-list, an invalid email fails the domain gate and
		// short-circuits; nil db proves no DB hit.
		err := checkUserEligibility(context.Background(), nil, "not-an-email", []string{"good.com"})
		if !errors.Is(err, errEmailDomainNotPermitted) {
			t.Fatalf("err = %v, want errEmailDomainNotPermitted", err)
		}
	})
}
