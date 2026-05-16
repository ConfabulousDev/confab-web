package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieSecure_HonorsInsecureDevMode(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want bool
	}{
		{"unset defaults to secure", "", true},
		{"true disables secure", "true", false},
		{"false keeps secure", "false", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("INSECURE_DEV_MODE", c.env)
			if got := cookieSecure(); got != c.want {
				t.Errorf("cookieSecure() = %v, want %v (INSECURE_DEV_MODE=%q)", got, c.want, c.env)
			}
		})
	}
}

func TestClearCookieWritesExpiredCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	clearCookie(rec, "test_cookie")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "test_cookie" {
		t.Errorf("Name = %q, want test_cookie", c.Name)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
	if c.Value != "" {
		t.Errorf("Value = %q, want empty", c.Value)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want SameSiteLaxMode", c.SameSite)
	}
}

func TestHandleCLIRedirect(t *testing.T) {
	cases := []struct {
		name         string
		cookieValue  string // empty means no cookie
		wantRedirect bool
		wantLocation string
	}{
		{
			name:         "no cookie returns false",
			cookieValue:  "",
			wantRedirect: false,
		},
		{
			name:         "valid /auth/cli/ prefix redirects",
			cookieValue:  "/auth/cli/abc",
			wantRedirect: true,
			wantLocation: "/auth/cli/abc",
		},
		{
			name:         "rejects open redirect to external URL",
			cookieValue:  "https://evil.com/phish",
			wantRedirect: false,
		},
		{
			name:         "rejects substring match (must be prefix)",
			cookieValue:  "https://evil.com/auth/cli/abc",
			wantRedirect: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			if c.cookieValue != "" {
				req.AddCookie(&http.Cookie{Name: "cli_redirect", Value: c.cookieValue})
			}

			got := handleCLIRedirect(rec, req, http.StatusTemporaryRedirect)
			if got != c.wantRedirect {
				t.Errorf("handleCLIRedirect = %v, want %v", got, c.wantRedirect)
			}
			if c.wantRedirect {
				if rec.Code != http.StatusTemporaryRedirect {
					t.Errorf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
				}
				if loc := rec.Header().Get("Location"); loc != c.wantLocation {
					t.Errorf("Location = %q, want %q", loc, c.wantLocation)
				}
			} else if rec.Code == http.StatusTemporaryRedirect {
				t.Error("no redirect should be written")
			}
		})
	}
}

func TestOAuthHTTPClient_HasTimeout(t *testing.T) {
	c := oauthHTTPClient()
	if c == nil {
		t.Fatal("oauthHTTPClient returned nil")
	}
	if c.Timeout != OAuthAPITimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, OAuthAPITimeout)
	}
	if c.Timeout == 0 {
		t.Error("Timeout should not be zero — risks hanging")
	}
}

func TestSetOAuthLoginCookies_HappyPath(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/login?email=user@example.com&redirect=/dashboard", nil)

	state, validEmail, expectedEmail, err := setOAuthLoginCookies(rec, req)
	if err != nil {
		t.Fatalf("setOAuthLoginCookies: %v", err)
	}
	if state == "" {
		t.Error("expected non-empty state")
	}
	if !validEmail {
		t.Error("expected validEmail=true for valid email hint")
	}
	if expectedEmail != "user@example.com" {
		t.Errorf("expectedEmail = %q, want user@example.com", expectedEmail)
	}

	names := map[string]bool{}
	for _, c := range rec.Result().Cookies() {
		names[c.Name] = true
	}
	for _, want := range []string{"oauth_state", "post_login_redirect", "expected_email"} {
		if !names[want] {
			t.Errorf("missing cookie: %s", want)
		}
	}
}

func TestSetOAuthLoginCookies_InvalidEmailHint(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/login?email=not-an-email", nil)

	_, validEmail, _, err := setOAuthLoginCookies(rec, req)
	if err != nil {
		t.Fatalf("setOAuthLoginCookies: %v", err)
	}
	if validEmail {
		t.Error("validEmail should be false for malformed email")
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "expected_email" {
			t.Error("expected_email cookie should not be emitted for invalid hint")
		}
	}
}

func TestSetOAuthLoginCookies_OmitsRedirectAndEmailWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/login", nil)

	state, validEmail, expectedEmail, err := setOAuthLoginCookies(rec, req)
	if err != nil {
		t.Fatalf("setOAuthLoginCookies: %v", err)
	}
	if state == "" {
		t.Error("state should always be set, even without redirect/email hint")
	}
	if validEmail {
		t.Error("validEmail should be false when email param absent")
	}
	if expectedEmail != "" {
		t.Errorf("expectedEmail = %q, want empty", expectedEmail)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "post_login_redirect" {
			t.Error("post_login_redirect cookie should not be emitted without ?redirect=")
		}
		if c.Name == "expected_email" {
			t.Error("expected_email cookie should not be emitted without ?email=")
		}
	}
}

func TestWriteDeviceTokenError_FormatAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	writeDeviceTokenError(rec, http.StatusBadRequest, "authorization_pending")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got DeviceTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if got.Error != "authorization_pending" {
		t.Errorf("error = %q, want authorization_pending", got.Error)
	}
	if got.AccessToken != "" {
		t.Errorf("AccessToken should be omitted on error, got %q", got.AccessToken)
	}
}

func TestSetOAuthLoginCookies_StateLengthAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/login", nil)
		state, _, _, err := setOAuthLoginCookies(rec, req)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[state] {
			t.Errorf("duplicate state token: %q", state)
		}
		seen[state] = true
		if len(state) < 16 {
			t.Errorf("state suspiciously short: %q", state)
		}
	}
}
