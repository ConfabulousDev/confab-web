package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// OIDCEndpoints holds the endpoints discovered from the OIDC provider
type OIDCEndpoints struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	Issuer                string `json:"issuer"`
}

// oidcUser represents user info from the OIDC userinfo endpoint
type oidcUser struct {
	Sub           string      `json:"sub"`
	Email         string      `json:"email"`
	EmailVerified interface{} `json:"email_verified"` // bool or string "true"
	Name          string      `json:"name"`
	Picture       string      `json:"picture"`
}

// IsEmailVerified returns true if email_verified is explicitly true.
// Handles both bool and string "true" representations.
// Missing/null email_verified is treated as unverified (strict mode).
func (u *oidcUser) IsEmailVerified() bool {
	switch v := u.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

// DiscoverOIDC fetches the OIDC discovery document from the issuer URL.
// Exported for testing.
func DiscoverOIDC(issuerURL string) (*OIDCEndpoints, error) {
	discoveryURL := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequest("GET", discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OIDC discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read discovery response: %w", err)
	}

	var endpoints OIDCEndpoints
	if err := json.Unmarshal(body, &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse OIDC discovery document: %w", err)
	}

	// Validate required endpoints
	if endpoints.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery: missing authorization_endpoint")
	}
	if endpoints.TokenEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery: missing token_endpoint")
	}
	if endpoints.UserinfoEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery: missing userinfo_endpoint")
	}

	// Validate issuer match (prevents confused deputy attacks)
	expectedIssuer := strings.TrimRight(issuerURL, "/")
	actualIssuer := strings.TrimRight(endpoints.Issuer, "/")
	if actualIssuer != expectedIssuer {
		return nil, fmt.Errorf("OIDC discovery: issuer mismatch: expected %q, got %q", expectedIssuer, actualIssuer)
	}

	return &endpoints, nil
}

// HandleOIDCLogin initiates the generic OIDC OAuth flow
func HandleOIDCLogin(config *OAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		frontendURL := os.Getenv("FRONTEND_URL")

		// Lazy discovery — if IdP is down, fail gracefully
		endpoints, err := config.getOIDCEndpoints()
		if err != nil {
			logger.Ctx(r.Context()).Error("OIDC discovery failed", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=oidc_error&error_description=%s",
				frontendURL,
				url.QueryEscape("SSO provider is temporarily unavailable. Please try again later."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		state, challenge, validEmail, expectedEmail, err := setOAuthLoginCookies(w, r)
		if err != nil {
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}

		authURL := fmt.Sprintf(
			"%s?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=%s",
			endpoints.AuthorizationEndpoint,
			url.QueryEscape(config.OIDCClientID),
			url.QueryEscape(config.OIDCRedirectURL),
			url.QueryEscape(state),
			url.QueryEscape("openid email profile"),
		)

		// PKCE (S256): bind the auth code to this browser's verifier cookie (r9zn).
		authURL += "&code_challenge=" + url.QueryEscape(challenge) + "&code_challenge_method=S256"

		// Add login hint if valid email is provided
		if validEmail {
			authURL += "&login_hint=" + url.QueryEscape(expectedEmail)
		}

		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleOIDCCallback handles the OAuth callback from the OIDC provider
func HandleOIDCCallback(config *OAuthConfig, database *db.DB) http.HandlerFunc {
	authStore := &dbauth.Store{DB: database}
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())
		ctx := r.Context()
		frontendURL := os.Getenv("FRONTEND_URL")

		// Validate state + PKCE verifier + code (shared across providers).
		code, codeVerifier, err := validateOAuthCallback(w, r)
		if err != nil {
			return
		}

		// Get discovered endpoints
		endpoints, err := config.getOIDCEndpoints()
		if err != nil {
			log.Error("OIDC discovery failed during callback", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=oidc_error&error_description=%s",
				frontendURL,
				url.QueryEscape("SSO provider is temporarily unavailable. Please try again later."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Exchange code for access token
		accessToken, err := exchangeOIDCCode(code, codeVerifier, config, endpoints)
		if err != nil {
			log.Error("Failed to exchange OIDC code", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=oidc_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to complete SSO authentication. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Get user info from OIDC userinfo endpoint
		user, err := getOIDCUser(accessToken, endpoints)
		if err != nil {
			log.Error("Failed to get OIDC user", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=oidc_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to retrieve user information from SSO provider. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// SECURITY: Strict email_verified check — missing = rejected
		if !user.IsEmailVerified() {
			log.Warn("OIDC email not verified", "email", user.Email, "sub", user.Sub)
			errorURL := fmt.Sprintf("%s/login?error=email_unverified&error_description=%s",
				frontendURL,
				url.QueryEscape("Your email is not verified by the SSO provider. Please verify your email and try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Normalize email to lowercase
		user.Email = strings.ToLower(user.Email)

		// Validate email format
		if !validation.IsValidEmail(user.Email) {
			log.Error("Invalid email from OIDC provider", "email", user.Email, "sub", user.Sub)
			errorURL := fmt.Sprintf("%s/login?error=oidc_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Invalid email received from SSO provider."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		log.Info("OIDC OAuth user retrieved",
			"sub", user.Sub,
			"email", user.Email,
			"name", user.Name)

		// CF-483: never let the demo email log in via OAuth.
		if IsDemoLoginEmail(config.DemoIdentityEmail, user.Email) {
			log.Warn("OIDC login attempt for demo identity rejected", "email", user.Email)
			redirectDemoLoginRejected(w, r, frontendURL)
			return
		}

		// Check email domain restriction + user cap (shared across providers).
		if err := checkUserEligibility(ctx, database, user.Email, config.AllowedEmailDomains); err != nil {
			redirectUserIneligible(w, r, frontendURL, "oidc", user.Email, err)
			return
		}

		// Find or create user in database
		oauthInfo := models.OAuthUserInfo{
			Provider:   models.ProviderOIDC,
			ProviderID: user.Sub,
			Email:      user.Email,
			Name:       user.Name,
			AvatarURL:  user.Picture,
		}
		dbUser, err := authStore.FindOrCreateUserByOAuth(ctx, oauthInfo, config.AutoLinkEmail)
		if err != nil {
			if errors.Is(err, db.ErrAutoLinkDisabled) {
				log.Warn("OAuth auto-link disabled; refusing to link to existing account", "email", oauthInfo.Email, "provider", "oidc")
				errorURL := fmt.Sprintf("%s/login?error=account_exists&error_description=%s",
					frontendURL,
					url.QueryEscape("An account with this email already exists. Sign in with your original method."))
				http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
				return
			}
			log.Error("Failed to create/find user in database", "error", err, "oidc_sub", user.Sub)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// w8tz: reject deactivated accounts BEFORE minting a session, so the
		// login loop (app→401→login→app) can never start for an inactive user.
		if dbUser.Status == models.UserStatusInactive {
			log.Warn("OAuth login blocked for inactive user", "email", user.Email, "provider", "oidc")
			redirectInactiveUser(w, r, frontendURL)
			return
		}

		// Create web session
		sessionID, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(SessionDuration)
		if err := authStore.CreateWebSession(ctx, sessionID, dbUser.ID, expiresAt); err != nil {
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    sessionID,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			Secure:   cookieSecure(),
			SameSite: http.SameSiteLaxMode,
		})

		// Handle email mismatch check and post-login redirect
		expectedEmail, emailMismatch := checkExpectedEmailMismatch(w, r, user.Email, "oidc")
		handlePostLoginRedirect(w, r, frontendURL, user.Email, expectedEmail, emailMismatch)
	}
}

// exchangeOIDCCode exchanges an authorization code for an access token
func exchangeOIDCCode(code, codeVerifier string, config *OAuthConfig, endpoints *OIDCEndpoints) (string, error) {
	data := url.Values{
		"client_id":     {config.OIDCClientID},
		"client_secret": {config.OIDCClientSecret},
		"code":          {code},
		"redirect_uri":  {config.OIDCRedirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier}, // PKCE (r9zn)
	}

	resp, err := oauthHTTPClient().PostForm(endpoints.TokenEndpoint, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading OIDC token response: %w", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("OIDC token error: %s - %s", result.Error, result.ErrorDesc)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in OIDC token response")
	}

	return result.AccessToken, nil
}

// getOIDCUser fetches user info from the OIDC userinfo endpoint
func getOIDCUser(accessToken string, endpoints *OIDCEndpoints) (*oidcUser, error) {
	req, err := http.NewRequest("GET", endpoints.UserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC userinfo returned status %d", resp.StatusCode)
	}

	var user oidcUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	if user.Sub == "" {
		return nil, fmt.Errorf("OIDC userinfo: missing sub claim")
	}

	if user.Email == "" {
		return nil, fmt.Errorf("OIDC userinfo: missing email claim")
	}

	return &user, nil
}
