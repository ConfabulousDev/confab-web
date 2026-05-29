package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/updatecheck"
)

// fakeChecker is a test double that returns a canned Status.
type fakeChecker struct{ s updatecheck.Status }

func (f fakeChecker) Status(_ context.Context) updatecheck.Status { return f.s }

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

func TestHandleAuthConfigVersion(t *testing.T) {
	t.Run("update available is wired through verbatim", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
			updateChecker: fakeChecker{s: updatecheck.Status{
				Current:         "v0.4.1",
				Latest:          "v0.5.0",
				LatestURL:       "https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0",
				UpdateAvailable: true,
			}},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Version.Current != "v0.4.1" {
			t.Errorf("version.current = %q, want %q", resp.Version.Current, "v0.4.1")
		}
		if resp.Version.Latest != "v0.5.0" {
			t.Errorf("version.latest = %q, want %q", resp.Version.Latest, "v0.5.0")
		}
		if resp.Version.LatestURL != "https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0" {
			t.Errorf("version.latest_url = %q, want the GitHub tag URL", resp.Version.LatestURL)
		}
		if !resp.Version.UpdateAvailable {
			t.Error("version.update_available = false, want true")
		}
		if resp.Version.UpdateCheckDisabled {
			t.Error("version.update_check_disabled = true, want false")
		}
		if resp.Version.UpdateCheckFailed {
			t.Error("version.update_check_failed = true, want false")
		}
	})

	t.Run("disabled checker surfaces update_check_disabled", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
			updateChecker: fakeChecker{s: updatecheck.Status{
				Current:             "v0.4.1",
				UpdateCheckDisabled: true,
			}},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Version.UpdateCheckDisabled {
			t.Error("version.update_check_disabled = false, want true")
		}
		if resp.Version.UpdateAvailable {
			t.Error("version.update_available = true, want false when disabled")
		}
		if resp.Version.Latest != "" {
			t.Errorf("version.latest = %q, want empty when disabled", resp.Version.Latest)
		}
	})

	t.Run("failed checker surfaces update_check_failed", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
			updateChecker: fakeChecker{s: updatecheck.Status{
				Current:           "v0.4.1",
				UpdateCheckFailed: true,
			}},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Version.UpdateCheckFailed {
			t.Error("version.update_check_failed = false, want true")
		}
		if resp.Version.UpdateAvailable {
			t.Error("version.update_available = true, want false on failure")
		}
	})

	t.Run("nil checker is treated as disabled (no panic)", func(t *testing.T) {
		s := &Server{
			oauthConfig:   &auth.OAuthConfig{},
			updateChecker: nil,
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with nil checker, got %d", rr.Code)
		}
		var resp authConfigResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Version.UpdateCheckDisabled {
			t.Error("nil checker should be reported as update_check_disabled=true")
		}
		if resp.Version.UpdateAvailable {
			t.Error("nil checker should not claim update_available=true")
		}
	})

	t.Run("response JSON has version object alongside features/providers", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
			updateChecker: fakeChecker{s: updatecheck.Status{
				Current:         "v0.4.1",
				Latest:          "v0.5.0",
				LatestURL:       "https://example.test/r",
				UpdateAvailable: true,
			}},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		body := rr.Body.String()
		for _, key := range []string{`"version"`, `"current":"v0.4.1"`, `"latest":"v0.5.0"`, `"update_available":true`} {
			if !strings.Contains(body, key) {
				t.Errorf("response body missing %s; got: %s", key, body)
			}
		}
	})

	t.Run("update_severity serializes verbatim when set", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
			updateChecker: fakeChecker{s: updatecheck.Status{
				Current:         "v0.4.1",
				Latest:          "v0.5.0",
				LatestURL:       "https://example.test/r",
				UpdateAvailable: true,
				UpdateSeverity:  "recommended",
			}},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		body := rr.Body.String()
		var resp authConfigResponse
		if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Version.UpdateSeverity != "recommended" {
			t.Errorf("version.update_severity = %q, want %q", resp.Version.UpdateSeverity, "recommended")
		}
		if !strings.Contains(body, `"update_severity":"recommended"`) {
			t.Errorf("response body missing update_severity; got: %s", body)
		}
	})

	t.Run("update_severity is omitted from the wire when empty", func(t *testing.T) {
		s := &Server{
			oauthConfig: &auth.OAuthConfig{},
			updateChecker: fakeChecker{s: updatecheck.Status{
				Current:         "v0.5.0",
				Latest:          "v0.5.0",
				UpdateAvailable: false,
			}},
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/config", nil)
		rr := httptest.NewRecorder()

		s.handleAuthConfig(rr, req)

		if strings.Contains(rr.Body.String(), "update_severity") {
			t.Errorf("update_severity should be omitted when empty; got: %s", rr.Body.String())
		}
	})
}
