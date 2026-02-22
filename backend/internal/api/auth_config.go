package api

import "net/http"

type providerInfo struct {
	Name        string `json:"name"`         // "github", "google", "oidc", "password"
	DisplayName string `json:"display_name"` // "GitHub", "Google", "Okta", "Password"
	LoginURL    string `json:"login_url"`    // "/auth/github/login", etc.
}

type authConfigFeatures struct {
	SharesEnabled    bool   `json:"shares_enabled"`
	SaasFooterEnabled bool   `json:"saas_footer_enabled"`
	SaasTermlyEnabled bool   `json:"saas_termly_enabled"`
	SupportEmail     string `json:"support_email"`
}

type authConfigResponse struct {
	Providers []providerInfo    `json:"providers"`
	Features  authConfigFeatures `json:"features"`
}

// handleAuthConfig returns the list of enabled authentication providers.
// Public endpoint — no authentication required.
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

	respondJSON(w, http.StatusOK, authConfigResponse{
		Providers: providers,
		Features: authConfigFeatures{
			SharesEnabled:    !s.sharesDisabled,
			SaasFooterEnabled: s.saasFooterEnabled,
			SaasTermlyEnabled: s.saasTermlyEnabled,
			SupportEmail:     s.supportEmail,
		},
	})
}
