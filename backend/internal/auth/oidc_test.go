package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDiscoverOIDC tests OIDC discovery endpoint fetching
func TestDiscoverOIDC(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/.well-known/openid-configuration" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 serverURL,
				"authorization_endpoint": serverURL + "/authorize",
				"token_endpoint":         serverURL + "/token",
				"userinfo_endpoint":      serverURL + "/userinfo",
			})
		}))
		defer server.Close()
		serverURL = server.URL

		endpoints, err := DiscoverOIDC(server.URL)
		if err != nil {
			t.Fatalf("DiscoverOIDC failed: %v", err)
		}

		if endpoints.AuthorizationEndpoint != server.URL+"/authorize" {
			t.Errorf("authorization_endpoint = %q, want %q", endpoints.AuthorizationEndpoint, server.URL+"/authorize")
		}
		if endpoints.TokenEndpoint != server.URL+"/token" {
			t.Errorf("token_endpoint = %q, want %q", endpoints.TokenEndpoint, server.URL+"/token")
		}
		if endpoints.UserinfoEndpoint != server.URL+"/userinfo" {
			t.Errorf("userinfo_endpoint = %q, want %q", endpoints.UserinfoEndpoint, server.URL+"/userinfo")
		}
		if endpoints.Issuer != server.URL {
			t.Errorf("issuer = %q, want %q", endpoints.Issuer, server.URL)
		}
	})

	t.Run("missing authorization_endpoint", func(t *testing.T) {
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":            serverURL,
				"token_endpoint":    serverURL + "/token",
				"userinfo_endpoint": serverURL + "/userinfo",
			})
		}))
		defer server.Close()
		serverURL = server.URL

		_, err := DiscoverOIDC(server.URL)
		if err == nil {
			t.Fatal("expected error for missing authorization_endpoint")
		}
		if !containsSubstring(err.Error(), "missing authorization_endpoint") {
			t.Errorf("error = %q, want mention of missing authorization_endpoint", err.Error())
		}
	})

	t.Run("missing token_endpoint", func(t *testing.T) {
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 serverURL,
				"authorization_endpoint": serverURL + "/authorize",
				"userinfo_endpoint":      serverURL + "/userinfo",
			})
		}))
		defer server.Close()
		serverURL = server.URL

		_, err := DiscoverOIDC(server.URL)
		if err == nil {
			t.Fatal("expected error for missing token_endpoint")
		}
		if !containsSubstring(err.Error(), "missing token_endpoint") {
			t.Errorf("error = %q, want mention of missing token_endpoint", err.Error())
		}
	})

	t.Run("missing userinfo_endpoint", func(t *testing.T) {
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 serverURL,
				"authorization_endpoint": serverURL + "/authorize",
				"token_endpoint":         serverURL + "/token",
			})
		}))
		defer server.Close()
		serverURL = server.URL

		_, err := DiscoverOIDC(server.URL)
		if err == nil {
			t.Fatal("expected error for missing userinfo_endpoint")
		}
		if !containsSubstring(err.Error(), "missing userinfo_endpoint") {
			t.Errorf("error = %q, want mention of missing userinfo_endpoint", err.Error())
		}
	})

	t.Run("issuer mismatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 "https://wrong-issuer.example.com",
				"authorization_endpoint": "https://wrong-issuer.example.com/authorize",
				"token_endpoint":         "https://wrong-issuer.example.com/token",
				"userinfo_endpoint":      "https://wrong-issuer.example.com/userinfo",
			})
		}))
		defer server.Close()

		_, err := DiscoverOIDC(server.URL)
		if err == nil {
			t.Fatal("expected error for issuer mismatch")
		}
		if !containsSubstring(err.Error(), "issuer mismatch") {
			t.Errorf("error = %q, want mention of issuer mismatch", err.Error())
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		_, err := DiscoverOIDC("http://127.0.0.1:1") // port 1 should be unreachable
		if err == nil {
			t.Fatal("expected error for unreachable server")
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := DiscoverOIDC(server.URL)
		if err == nil {
			t.Fatal("expected error for non-200 status")
		}
		if !containsSubstring(err.Error(), "status 404") {
			t.Errorf("error = %q, want mention of status 404", err.Error())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		_, err := DiscoverOIDC(server.URL)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("trailing slash normalization", func(t *testing.T) {
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 serverURL + "/",
				"authorization_endpoint": serverURL + "/authorize",
				"token_endpoint":         serverURL + "/token",
				"userinfo_endpoint":      serverURL + "/userinfo",
			})
		}))
		defer server.Close()
		serverURL = server.URL

		// Should succeed even if issuer has trailing slash
		endpoints, err := DiscoverOIDC(server.URL + "/")
		if err != nil {
			t.Fatalf("DiscoverOIDC with trailing slash failed: %v", err)
		}
		if endpoints.AuthorizationEndpoint != server.URL+"/authorize" {
			t.Errorf("unexpected authorization_endpoint: %s", endpoints.AuthorizationEndpoint)
		}
	})
}

// TestOIDCUser_IsEmailVerified tests email_verified handling
func TestOIDCUser_IsEmailVerified(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string True", "True", true},
		{"string TRUE", "TRUE", true},
		{"string false", "false", false},
		{"string empty", "", false},
		{"nil", nil, false},
		{"number", float64(1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &OIDCUser{EmailVerified: tt.value}
			if got := user.IsEmailVerified(); got != tt.expected {
				t.Errorf("IsEmailVerified() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHandleOIDCLogin_StateGeneration tests that OIDC login generates state and redirects
func TestHandleOIDCLogin_StateGeneration(t *testing.T) {
	// Set up a mock OIDC discovery server
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 serverURL,
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"userinfo_endpoint":      serverURL + "/userinfo",
		})
	}))
	defer server.Close()
	serverURL = server.URL

	config := &OAuthConfig{
		OIDCEnabled:     true,
		OIDCClientID:    "test_client_id",
		OIDCRedirectURL: "http://localhost:8080/auth/oidc/callback",
		OIDCIssuerURL:   server.URL,
		OIDCDisplayName: "TestIDP",
	}

	handler := HandleOIDCLogin(config)

	req := httptest.NewRequest("GET", "/auth/oidc/login", nil)
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

	if stateCookie.Value == "" {
		t.Error("oauth_state cookie value is empty")
	}

	if !stateCookie.HttpOnly {
		t.Error("oauth_state cookie should be HttpOnly")
	}

	// Redirect URL should go to discovered authorization endpoint
	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header not set")
	}

	if !containsSubstring(location, server.URL+"/authorize") {
		t.Errorf("redirect location doesn't contain discovered auth endpoint: %s", location)
	}

	// State in URL should match cookie
	if !containsSubstring(location, "state="+stateCookie.Value) {
		t.Error("redirect URL state doesn't match cookie state")
	}

	// Should include openid email profile scope
	if !containsSubstring(location, "scope=openid") {
		t.Errorf("redirect URL missing openid scope: %s", location)
	}
}

// TestHandleOIDCLogin_EmailHint tests OIDC login hint with email
func TestHandleOIDCLogin_EmailHint(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 serverURL,
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"userinfo_endpoint":      serverURL + "/userinfo",
		})
	}))
	defer server.Close()
	serverURL = server.URL

	config := &OAuthConfig{
		OIDCEnabled:     true,
		OIDCClientID:    "test_client_id",
		OIDCRedirectURL: "http://localhost:8080/auth/oidc/callback",
		OIDCIssuerURL:   server.URL,
		OIDCDisplayName: "TestIDP",
	}

	t.Run("valid email adds login_hint", func(t *testing.T) {
		handler := HandleOIDCLogin(config)

		req := httptest.NewRequest("GET", "/auth/oidc/login?email=user@example.com", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		if !containsSubstring(location, "login_hint=user%40example.com") {
			t.Errorf("redirect URL missing login_hint: %s", location)
		}

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
		if emailCookie.Value != "user@example.com" {
			t.Errorf("expected_email = %q, want 'user@example.com'", emailCookie.Value)
		}
	})

	t.Run("invalid email does not add login_hint", func(t *testing.T) {
		handler := HandleOIDCLogin(config)

		req := httptest.NewRequest("GET", "/auth/oidc/login?email=not-valid", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		location := rec.Header().Get("Location")
		if containsSubstring(location, "login_hint=") {
			t.Errorf("redirect URL should not have login_hint for invalid email: %s", location)
		}
	})
}

// TestHandleOIDCLogin_DiscoveryFailure tests graceful error when IdP is unreachable
func TestHandleOIDCLogin_DiscoveryFailure(t *testing.T) {
	t.Setenv("FRONTEND_URL", "http://localhost:3000")

	config := &OAuthConfig{
		OIDCEnabled:     true,
		OIDCClientID:    "test_client_id",
		OIDCRedirectURL: "http://localhost:8080/auth/oidc/callback",
		OIDCIssuerURL:   "http://127.0.0.1:1", // unreachable
		OIDCDisplayName: "TestIDP",
	}

	handler := HandleOIDCLogin(config)

	req := httptest.NewRequest("GET", "/auth/oidc/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should redirect to frontend with error
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	location := rec.Header().Get("Location")
	if !containsSubstring(location, "error=oidc_error") {
		t.Errorf("redirect URL should contain error=oidc_error: %s", location)
	}
	if !containsSubstring(location, "localhost:3000") {
		t.Errorf("redirect URL should go to frontend: %s", location)
	}
}

// TestHandleOIDCCallback_CSRFValidation tests CSRF state validation for OIDC
func TestHandleOIDCCallback_CSRFValidation(t *testing.T) {
	config := &OAuthConfig{
		OIDCEnabled:      true,
		OIDCClientID:     "test_client_id",
		OIDCClientSecret: "test_client_secret",
		OIDCRedirectURL:  "http://localhost:8080/auth/oidc/callback",
		OIDCIssuerURL:    "https://example.okta.com",
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
		{
			name:           "empty state query",
			stateCookie:    "valid_state",
			stateQuery:     "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := HandleOIDCCallback(config, nil) // nil db - won't reach it

			req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+tt.stateQuery+"&code=test_code", nil)
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
			if body != "Invalid state parameter\n" {
				t.Errorf("body = %q, want 'Invalid state parameter'", body)
			}
		})
	}
}

// TestHandleOIDCCallback_MissingCode tests missing code parameter for OIDC
func TestHandleOIDCCallback_MissingCode(t *testing.T) {
	config := &OAuthConfig{
		OIDCEnabled:      true,
		OIDCClientID:     "test_client_id",
		OIDCClientSecret: "test_client_secret",
		OIDCRedirectURL:  "http://localhost:8080/auth/oidc/callback",
		OIDCIssuerURL:    "https://example.okta.com",
	}

	handler := HandleOIDCCallback(config, nil)

	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=valid_state", nil)
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

// TestHandleLoginSelector_OIDCButton tests OIDC button rendering in login selector
func TestHandleLoginSelector_OIDCButton(t *testing.T) {
	t.Run("OIDC button with custom display name", func(t *testing.T) {
		config := OAuthConfig{
			OIDCEnabled:     true,
			OIDCDisplayName: "Okta",
		}

		handler := HandleLoginSelector(config)

		req := httptest.NewRequest("GET", "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if !containsSubstring(body, "Continue with Okta") {
			t.Error("page should display 'Continue with Okta'")
		}
		if !containsSubstring(body, "/auth/oidc/login") {
			t.Error("page should contain OIDC login link")
		}
		if !containsSubstring(body, "btn-oidc") {
			t.Error("page should contain btn-oidc class")
		}
	})

	t.Run("OIDC button with default display name", func(t *testing.T) {
		config := OAuthConfig{
			OIDCEnabled: true,
		}

		handler := HandleLoginSelector(config)

		req := httptest.NewRequest("GET", "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if !containsSubstring(body, "Continue with SSO") {
			t.Error("page should display 'Continue with SSO' as default")
		}
	})

	t.Run("OIDC button HTML-escapes display name", func(t *testing.T) {
		config := OAuthConfig{
			OIDCEnabled:     true,
			OIDCDisplayName: `<script>alert("xss")</script>`,
		}

		handler := HandleLoginSelector(config)

		req := httptest.NewRequest("GET", "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if containsSubstring(body, "<script>") {
			t.Error("XSS attempt should be escaped")
		}
		if !containsSubstring(body, "&lt;script&gt;") {
			t.Error("display name should be HTML-escaped")
		}
	})

	t.Run("OIDC button not shown when disabled", func(t *testing.T) {
		config := OAuthConfig{
			GitHubEnabled: true,
		}

		handler := HandleLoginSelector(config)

		req := httptest.NewRequest("GET", "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if containsSubstring(body, "/auth/oidc/login") {
			t.Error("page should not contain OIDC login link when disabled")
		}
	})

	t.Run("OIDC button shown alongside GitHub and Google", func(t *testing.T) {
		config := OAuthConfig{
			GitHubEnabled:   true,
			GoogleEnabled:   true,
			OIDCEnabled:     true,
			OIDCDisplayName: "Okta",
		}

		handler := HandleLoginSelector(config)

		req := httptest.NewRequest("GET", "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if !containsSubstring(body, "Continue with GitHub") {
			t.Error("should show GitHub button")
		}
		if !containsSubstring(body, "Continue with Google") {
			t.Error("should show Google button")
		}
		if !containsSubstring(body, "Continue with Okta") {
			t.Error("should show OIDC button")
		}
	})

	t.Run("divider shown with password + OIDC only", func(t *testing.T) {
		config := OAuthConfig{
			PasswordEnabled: true,
			OIDCEnabled:     true,
			OIDCDisplayName: "SSO",
		}

		handler := HandleLoginSelector(config)

		req := httptest.NewRequest("GET", "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if !containsSubstring(body, "or continue with") {
			t.Error("divider should appear with password + OIDC")
		}
	})
}
