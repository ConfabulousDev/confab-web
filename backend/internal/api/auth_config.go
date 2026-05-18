package api

import (
	"context"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/updatecheck"
)

type providerInfo struct {
	Name        string `json:"name"`         // "github", "google", "oidc", "password"
	DisplayName string `json:"display_name"` // "GitHub", "Google", "Okta", "Password"
	LoginURL    string `json:"login_url"`    // "/auth/github/login", etc.
}

type authConfigFeatures struct {
	SharesEnabled       bool   `json:"shares_enabled"`
	SaasFooterEnabled   bool   `json:"saas_footer_enabled"`
	SaasTermlyEnabled   bool   `json:"saas_termly_enabled"`
	OrgAnalyticsEnabled bool   `json:"org_analytics_enabled"`
	PasswordAuthEnabled bool   `json:"password_auth_enabled"`
	SmartRecapEnabled   bool   `json:"smart_recap_enabled"`
	SupportEmail        string `json:"support_email"`
}

// UpdateChecker is the contract handleAuthConfig needs from updatecheck. The
// interface lets tests inject a canned Status without GitHub roundtrips.
type UpdateChecker interface {
	Status(ctx context.Context) updatecheck.Status
}

type authConfigResponse struct {
	Providers []providerInfo     `json:"providers"`
	Features  authConfigFeatures `json:"features"`
	Version   updatecheck.Status `json:"version"`
}

// handleAuthConfig returns enabled auth providers, SaaS feature flags, and
// the running backend version. Public endpoint — no authentication required.
func (s *Server) handleAuthConfig(w http.ResponseWriter, r *http.Request) {
	providers := []providerInfo{}

	if s.oauthConfig.PasswordEnabled {
		providers = append(providers, providerInfo{
			Name:        "password",
			DisplayName: "Password",
			LoginURL:    "/auth/password/login",
		})
	}

	if s.oauthConfig.GitHubEnabled {
		providers = append(providers, providerInfo{
			Name:        "github",
			DisplayName: "GitHub",
			LoginURL:    "/auth/github/login",
		})
	}

	if s.oauthConfig.GoogleEnabled {
		providers = append(providers, providerInfo{
			Name:        "google",
			DisplayName: "Google",
			LoginURL:    "/auth/google/login",
		})
	}

	if s.oauthConfig.OIDCEnabled {
		displayName := s.oauthConfig.OIDCDisplayName
		if displayName == "" {
			displayName = "SSO"
		}
		providers = append(providers, providerInfo{
			Name:        "oidc",
			DisplayName: displayName,
			LoginURL:    "/auth/oidc/login",
		})
	}

	version := updatecheck.Status{UpdateCheckDisabled: true}
	if s.updateChecker != nil {
		version = s.updateChecker.Status(r.Context())
	}

	respondJSON(w, http.StatusOK, authConfigResponse{
		Providers: providers,
		Features: authConfigFeatures{
			SharesEnabled:       s.sharesEnabled,
			SaasFooterEnabled:   s.saasFooterEnabled,
			SaasTermlyEnabled:   s.saasTermlyEnabled,
			OrgAnalyticsEnabled: s.orgAnalyticsEnabled,
			PasswordAuthEnabled: s.oauthConfig.PasswordEnabled,
			SmartRecapEnabled:   s.smartRecapEnabled,
			SupportEmail:        s.supportEmail,
		},
		Version: version,
	})
}
