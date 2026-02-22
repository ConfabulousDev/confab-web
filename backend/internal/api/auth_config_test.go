package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
)

func TestHandleAuthConfig(t *testing.T) {
	t.Run("no providers enabled", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}

		// Verify JSON contains empty array, not null (frontend would crash on null)
		body := rr.Body.String()
		if !strings.Contains(body, `"providers":[]`) {
			t.Errorf("expected providers to be empty array [], got: %s", body)
		}

		var resp authConfigResponse
		if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Providers) != 0 {
			t.Errorf("expected 0 providers, got %d", len(resp.Providers))
		}
	})

	t.Run("all providers enabled", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{
				PasswordEnabled: true,
				GitHubEnabled:   true,
				GoogleEnabled:   true,
				OIDCEnabled:     true,
				OIDCDisplayName: "Okta",
			},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Providers) != 4 {
			t.Fatalf("expected 4 providers, got %d", len(resp.Providers))
		}

		// Verify order: password, github, google, oidc
		expected := []struct {
			name        string
			displayName string
			loginURL    string
		}{
			{"password", "Password", "/auth/password/login"},
			{"github", "GitHub", "/auth/github/login"},
			{"google", "Google", "/auth/google/login"},
			{"oidc", "Okta", "/auth/oidc/login"},
		}
		for i, e := range expected {
			if resp.Providers[i].Name != e.name {
				t.Errorf("provider[%d].name = %q, want %q", i, resp.Providers[i].Name, e.name)
			}
			if resp.Providers[i].DisplayName != e.displayName {
				t.Errorf("provider[%d].display_name = %q, want %q", i, resp.Providers[i].DisplayName, e.displayName)
			}
			if resp.Providers[i].LoginURL != e.loginURL {
				t.Errorf("provider[%d].login_url = %q, want %q", i, resp.Providers[i].LoginURL, e.loginURL)
			}
		}
	})

	t.Run("github only", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{
				GitHubEnabled: true,
			},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Providers) != 1 {
			t.Fatalf("expected 1 provider, got %d", len(resp.Providers))
		}
		if resp.Providers[0].Name != "github" {
			t.Errorf("expected github, got %q", resp.Providers[0].Name)
		}
	})

	t.Run("OIDC defaults display name to SSO", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{
				OIDCEnabled: true,
				// OIDCDisplayName left empty
			},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Providers) != 1 {
			t.Fatalf("expected 1 provider, got %d", len(resp.Providers))
		}
		if resp.Providers[0].DisplayName != "SSO" {
			t.Errorf("expected display_name SSO, got %q", resp.Providers[0].DisplayName)
		}
	})

	t.Run("password and google", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{
				PasswordEnabled: true,
				GoogleEnabled:   true,
			},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Providers) != 2 {
			t.Fatalf("expected 2 providers, got %d", len(resp.Providers))
		}
		if resp.Providers[0].Name != "password" {
			t.Errorf("expected password first, got %q", resp.Providers[0].Name)
		}
		if resp.Providers[1].Name != "google" {
			t.Errorf("expected google second, got %q", resp.Providers[1].Name)
		}
	})

	t.Run("response has correct content type", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{
				GitHubEnabled: true,
			},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		cc := rr.Header().Get("Cache-Control")
		if cc != "no-store" {
			t.Errorf("expected Cache-Control no-store, got %q", cc)
		}
	})

	t.Run("features shares_enabled defaults to false", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{
				GitHubEnabled: true,
			},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Features.SharesEnabled {
			t.Error("expected features.shares_enabled to be false by default")
		}
	})

	t.Run("features saas_footer_enabled defaults to false", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Features.SaasFooterEnabled {
			t.Error("expected features.saas_footer_enabled to be false by default")
		}
	})

	t.Run("features saas_footer_enabled true when enabled", func(t *testing.T) {
		s := &Server{
			oauthConfig:       &auth.OAuthConfig{},
			saasFooterEnabled: true,
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Features.SaasFooterEnabled {
			t.Error("expected features.saas_footer_enabled to be true when enabled")
		}
	})

	t.Run("features saas_termly_enabled defaults to false", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Features.SaasTermlyEnabled {
			t.Error("expected features.saas_termly_enabled to be false by default")
		}
	})

	t.Run("features saas_termly_enabled true when enabled", func(t *testing.T) {
		s := &Server{
			oauthConfig:       &auth.OAuthConfig{},
			saasTermlyEnabled: true,
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Features.SaasTermlyEnabled {
			t.Error("expected features.saas_termly_enabled to be true when enabled")
		}
	})

	t.Run("features shares_enabled false when not enabled", func(t *testing.T) {
		s := &Server{
			oauthConfig:   &auth.OAuthConfig{},
			sharesEnabled: false,
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Features.SharesEnabled {
			t.Error("expected features.shares_enabled to be false when not enabled")
		}
	})

	t.Run("features shares_enabled true when enabled", func(t *testing.T) {
		s := &Server{
			oauthConfig:   &auth.OAuthConfig{},
			sharesEnabled: true,
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Features.SharesEnabled {
			t.Error("expected features.shares_enabled to be true when enabled")
		}
	})
}
