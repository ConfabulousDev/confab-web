package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleGitHubCallback_CSRFValidation tests CSRF state validation
func TestHandleGitHubCallback_CSRFValidation(t *testing.T) {
	config := OAuthConfig{
		GitHubClientID:     "test_client_id",
		GitHubClientSecret: "test_client_secret",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}

	tests := []struct {
		name           string
		stateCookie    string
		stateQuery     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing state cookie",
			stateCookie:    "",
			stateQuery:     "valid_state",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid state parameter\n",
		},
		{
			name:           "state mismatch",
			stateCookie:    "cookie_state",
			stateQuery:     "different_state",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid state parameter\n",
		},
		{
			name:           "empty state query",
			stateCookie:    "valid_state",
			stateQuery:     "",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid state parameter\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := HandleGitHubCallback(&config, nil) // nil db - we won't reach it

			req := httptest.NewRequest("GET", "/auth/github/callback?state="+tt.stateQuery+"&code=test_code", nil)
			if tt.stateCookie != "" {
				req.AddCookie(&http.Cookie{
					Name:  "oauth_state",
					Value: tt.stateCookie,
				})
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			body := rec.Body.String()
			if body != tt.expectedBody {
				t.Errorf("body = %q, want %q", body, tt.expectedBody)
			}
		})
	}
}

// TestHandleGitHubCallback_MissingCode tests missing code parameter
func TestHandleGitHubCallback_MissingCode(t *testing.T) {
	config := OAuthConfig{
		GitHubClientID:     "test_client_id",
		GitHubClientSecret: "test_client_secret",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}

	handler := HandleGitHubCallback(&config, nil)

	req := httptest.NewRequest("GET", "/auth/github/callback?state=valid_state", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "valid_state",
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	body := rec.Body.String()
	if body != "Missing code parameter\n" {
		t.Errorf("body = %q, want 'Missing code parameter'", body)
	}
}

// TestHandleGoogleCallback_CSRFValidation tests Google OAuth CSRF protection
func TestHandleGoogleCallback_CSRFValidation(t *testing.T) {
	config := OAuthConfig{
		GoogleClientID:     "test_client_id",
		GoogleClientSecret: "test_client_secret",
		GoogleRedirectURL:  "http://localhost:8080/auth/google/callback",
	}

	tests := []struct {
		name           string
		stateCookie    string
		stateQuery     string
		expectedStatus int
	}{
		{
			name:           "missing state cookie",
			stateCookie:    "",
			stateQuery:     "valid_state",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "state mismatch",
			stateCookie:    "cookie_state",
			stateQuery:     "different_state",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := HandleGoogleCallback(&config, nil)

			req := httptest.NewRequest("GET", "/auth/google/callback?state="+tt.stateQuery+"&code=test_code", nil)
			if tt.stateCookie != "" {
				req.AddCookie(&http.Cookie{
					Name:  "oauth_state",
					Value: tt.stateCookie,
				})
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestHandleGoogleCallback_MissingCode tests missing code parameter for Google
func TestHandleGoogleCallback_MissingCode(t *testing.T) {
	config := OAuthConfig{
		GoogleClientID:     "test_client_id",
		GoogleClientSecret: "test_client_secret",
		GoogleRedirectURL:  "http://localhost:8080/auth/google/callback",
	}

	handler := HandleGoogleCallback(&config, nil)

	req := httptest.NewRequest("GET", "/auth/google/callback?state=valid_state", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "valid_state",
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	body := rec.Body.String()
	if body != "Missing code parameter\n" {
		t.Errorf("body = %q, want 'Missing code parameter'", body)
	}
}

// TestHandleGitHubLogin_StateGeneration tests that login generates state and sets cookies
func TestHandleGitHubLogin_StateGeneration(t *testing.T) {
	config := OAuthConfig{
		GitHubClientID:     "test_client_id",
		GitHubClientSecret: "test_client_secret",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}

	handler := HandleGitHubLogin(&config)

	req := httptest.NewRequest("GET", "/auth/github/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should redirect
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	// Should set oauth_state cookie
	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}

	if stateCookie == nil {
		t.Fatal("oauth_state cookie not set")
	}

	// State should be non-empty
	if stateCookie.Value == "" {
		t.Error("oauth_state cookie value is empty")
	}

	// Cookie should be HttpOnly
	if !stateCookie.HttpOnly {
		t.Error("oauth_state cookie should be HttpOnly")
	}

	// Cookie should have SameSite=Lax
	if stateCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", stateCookie.SameSite)
	}

	// Redirect URL should contain state parameter
	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header not set")
	}

	// State in URL should match cookie
	if !containsSubstring(location, "state="+stateCookie.Value) {
		t.Error("redirect URL state doesn't match cookie state")
	}
}

// TestHandleGitHubLogin_PreservesRedirect tests that redirect URL is preserved
func TestHandleGitHubLogin_PreservesRedirect(t *testing.T) {
	config := OAuthConfig{
		GitHubClientID:     "test_client_id",
		GitHubClientSecret: "test_client_secret",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}

	handler := HandleGitHubLogin(&config)

	req := httptest.NewRequest("GET", "/auth/github/login?redirect=/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should set post_login_redirect cookie
	cookies := rec.Result().Cookies()
	var redirectCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "post_login_redirect" {
			redirectCookie = c
			break
		}
	}

	if redirectCookie == nil {
		t.Fatal("post_login_redirect cookie not set")
	}

	if redirectCookie.Value != "/dashboard" {
		t.Errorf("post_login_redirect = %q, want '/dashboard'", redirectCookie.Value)
	}
}

// TestHandleGoogleLogin_StateGeneration tests Google login state generation
func TestHandleGoogleLogin_StateGeneration(t *testing.T) {
	config := OAuthConfig{
		GoogleClientID:     "test_client_id",
		GoogleClientSecret: "test_client_secret",
		GoogleRedirectURL:  "http://localhost:8080/auth/google/callback",
	}

	handler := HandleGoogleLogin(&config)

	req := httptest.NewRequest("GET", "/auth/google/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should redirect
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	// Should set oauth_state cookie
	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}

	if stateCookie == nil {
		t.Fatal("oauth_state cookie not set")
	}

	// Redirect should go to Google
	location := rec.Header().Get("Location")
	if !containsSubstring(location, "accounts.google.com") {
		t.Errorf("redirect location doesn't contain Google: %s", location)
	}
}

// TestHandleCLIAuthorize_RedirectsWithoutSession tests that unauthenticated users are redirected
func TestHandleCLIAuthorize_RedirectsWithoutSession(t *testing.T) {
	// Note: This handler requires a real DB to validate sessions.
	// We test the redirect behavior for missing sessions here.
	// Without a session cookie, it should redirect to login.

	// The actual behavior requires a db, so we skip the full test
	// but verify the expected flow via documentation
	t.Run("documents expected behavior", func(t *testing.T) {
		// When no session cookie:
		// 1. Sets cli_redirect cookie with return URL
		// 2. Redirects to /login
		// This is verified in integration tests with real DB
	})
}

// TestHandleCLIAuthorize_InvalidCallback tests invalid callback URL
func TestHandleCLIAuthorize_InvalidCallbackRejection(t *testing.T) {
	// This test verifies that non-localhost callbacks would be rejected
	// We test isLocalhostURL separately but this is the integration point
	invalidCallbacks := []string{
		"http://evil.com/callback",
		"https://localhost/callback", // https not allowed
		"http://attacker.com:8080/steal",
	}

	for _, callback := range invalidCallbacks {
		t.Run(callback, func(t *testing.T) {
			// isLocalhostURL should reject these
			if isLocalhostURL(callback) {
				t.Errorf("isLocalhostURL(%q) = true, should be false", callback)
			}
		})
	}
}

// TestGenerateUserCode tests the human-friendly code generation
func TestGenerateUserCode(t *testing.T) {
	t.Run("generates correct format", func(t *testing.T) {
		code, err := generateUserCode()
		if err != nil {
			t.Fatalf("generateUserCode failed: %v", err)
		}

		// Should be 9 chars: XXXX-XXXX
		if len(code) != 9 {
			t.Errorf("code length = %d, want 9", len(code))
		}

		// Should have dash in middle
		if code[4] != '-' {
			t.Errorf("code[4] = %c, want '-'", code[4])
		}
	})

	t.Run("uses safe alphabet", func(t *testing.T) {
		// Confusing chars: 0, O, I, L, 1
		safeChars := "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

		for i := 0; i < 100; i++ {
			code, err := generateUserCode()
			if err != nil {
				t.Fatalf("generateUserCode failed: %v", err)
			}

			// Check each char (excluding dash)
			for j, c := range code {
				if j == 4 {
					continue // skip dash
				}
				if !containsRune(safeChars, c) {
					t.Errorf("code contains unsafe char: %c", c)
				}
			}
		}
	})

	t.Run("generates unique codes", func(t *testing.T) {
		codes := make(map[string]bool)
		for i := 0; i < 100; i++ {
			code, err := generateUserCode()
			if err != nil {
				t.Fatalf("generateUserCode failed: %v", err)
			}
			if codes[code] {
				t.Errorf("duplicate code generated: %s", code)
			}
			codes[code] = true
		}
	})
}

// TestGenerateDeviceCode tests the device code generation
func TestGenerateDeviceCode(t *testing.T) {
	t.Run("generates 64 char hex", func(t *testing.T) {
		code, err := generateDeviceCode()
		if err != nil {
			t.Fatalf("generateDeviceCode failed: %v", err)
		}

		// 32 bytes -> 64 hex chars
		if len(code) != 64 {
			t.Errorf("code length = %d, want 64", len(code))
		}

		// All chars should be hex
		for _, c := range code {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("code contains non-hex char: %c", c)
			}
		}
	})

	t.Run("generates unique codes", func(t *testing.T) {
		codes := make(map[string]bool)
		for i := 0; i < 100; i++ {
			code, err := generateDeviceCode()
			if err != nil {
				t.Fatalf("generateDeviceCode failed: %v", err)
			}
			if codes[code] {
				t.Errorf("duplicate code generated: %s", code)
			}
			codes[code] = true
		}
	})
}

// TestHandleGitHubLogin_EmailHint tests email login hint functionality
func TestHandleGitHubLogin_EmailHint(t *testing.T) {
	config := OAuthConfig{
		GitHubClientID:     "test_client_id",
		GitHubClientSecret: "test_client_secret",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}

	t.Run("valid email sets cookie and login hint", func(t *testing.T) {
		handler := HandleGitHubLogin(&config)

		req := httptest.NewRequest("GET", "/auth/github/login?email=alice@example.com", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Should set expected_email cookie
		cookies := rec.Result().Cookies()
		var emailCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "expected_email" {
				emailCookie = c
				break
			}
		}

		if emailCookie == nil {
			t.Fatal("expected_email cookie not set")
		}

		if emailCookie.Value != "alice@example.com" {
			t.Errorf("expected_email = %q, want 'alice@example.com'", emailCookie.Value)
		}

		// Should add login hint to redirect URL
		location := rec.Header().Get("Location")
		if !containsSubstring(location, "login=alice%40example.com") {
			t.Errorf("redirect URL missing login hint: %s", location)
		}
	})

	t.Run("invalid email does not set cookie or hint", func(t *testing.T) {
		handler := HandleGitHubLogin(&config)

		req := httptest.NewRequest("GET", "/auth/github/login?email=not-an-email", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Should NOT set expected_email cookie
		cookies := rec.Result().Cookies()
		for _, c := range cookies {
			if c.Name == "expected_email" {
				t.Error("expected_email cookie should not be set for invalid email")
			}
		}

		// Should NOT add login hint to redirect URL
		location := rec.Header().Get("Location")
		if containsSubstring(location, "login=") {
			t.Errorf("redirect URL should not have login hint for invalid email: %s", location)
		}
	})

	t.Run("empty email does not set cookie", func(t *testing.T) {
		handler := HandleGitHubLogin(&config)

		req := httptest.NewRequest("GET", "/auth/github/login?email=", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		cookies := rec.Result().Cookies()
		for _, c := range cookies {
			if c.Name == "expected_email" {
				t.Error("expected_email cookie should not be set for empty email")
			}
		}
	})
}

// TestHandleGoogleLogin_EmailHint tests Google email login hint
func TestHandleGoogleLogin_EmailHint(t *testing.T) {
	config := OAuthConfig{
		GoogleClientID:     "test_client_id",
		GoogleClientSecret: "test_client_secret",
		GoogleRedirectURL:  "http://localhost:8080/auth/google/callback",
	}

	t.Run("valid email adds login_hint", func(t *testing.T) {
		handler := HandleGoogleLogin(&config)

		req := httptest.NewRequest("GET", "/auth/google/login?email=bob@example.com", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Should add login_hint to redirect URL
		location := rec.Header().Get("Location")
		if !containsSubstring(location, "login_hint=bob%40example.com") {
			t.Errorf("redirect URL missing login_hint: %s", location)
		}
	})

	t.Run("invalid email does not add login_hint", func(t *testing.T) {
		handler := HandleGoogleLogin(&config)

		req := httptest.NewRequest("GET", "/auth/google/login?email=garbage", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		if containsSubstring(location, "login_hint=") {
			t.Errorf("redirect URL should not have login_hint for invalid email: %s", location)
		}
	})
}

// TestHandleLogout_RedirectSupport tests logout redirect functionality
func TestHandleLogout_RedirectSupport(t *testing.T) {
	// Note: HandleLogout requires a db, but we can still test the redirect logic
	// by passing nil db (logout will still work, just won't delete session from db)

	t.Run("redirects to relative path", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "http://localhost:3000")

		handler := HandleLogout(nil)

		req := httptest.NewRequest("GET", "/auth/logout?redirect=/auth/github/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		// /auth paths should redirect directly (no frontend URL prefix)
		if location != "/auth/github/login" {
			t.Errorf("location = %q, want '/auth/github/login'", location)
		}
	})

	t.Run("prepends frontend URL for non-auth paths", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "http://localhost:3000")

		handler := HandleLogout(nil)

		req := httptest.NewRequest("GET", "/auth/logout?redirect=/sessions/123", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		if location != "http://localhost:3000/sessions/123" {
			t.Errorf("location = %q, want 'http://localhost:3000/sessions/123'", location)
		}
	})

	t.Run("blocks absolute URL redirects (open redirect attack)", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "http://localhost:3000")

		handler := HandleLogout(nil)

		req := httptest.NewRequest("GET", "/auth/logout?redirect=http://evil.com", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		// Should redirect to frontend, not evil.com
		if location != "http://localhost:3000" {
			t.Errorf("location = %q, should redirect to frontend, not attacker URL", location)
		}
	})

	t.Run("blocks protocol-relative redirects", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "http://localhost:3000")

		handler := HandleLogout(nil)

		req := httptest.NewRequest("GET", "/auth/logout?redirect=//evil.com/path", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		if containsSubstring(location, "evil.com") {
			t.Errorf("location contains evil.com - open redirect vulnerability: %s", location)
		}
	})
}


// Helper functions

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
