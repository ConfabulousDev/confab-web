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

// googleUser represents Google user info from OAuth
type googleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// HandleGoogleLogin initiates Google OAuth flow
func HandleGoogleLogin(config *OAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, challenge, validEmail, expectedEmail, err := setOAuthLoginCookies(w, r)
		if err != nil {
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}

		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=%s",
			url.QueryEscape(config.GoogleClientID),
			url.QueryEscape(config.GoogleRedirectURL),
			url.QueryEscape(state),
			url.QueryEscape("openid email profile"),
		)

		// PKCE (S256): bind the auth code to this browser's verifier cookie (r9zn).
		authURL += "&code_challenge=" + url.QueryEscape(challenge) + "&code_challenge_method=S256"

		if validEmail {
			authURL += "&login_hint=" + url.QueryEscape(expectedEmail)
		}

		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleGoogleCallback handles the OAuth callback from Google.
// Cross-provider validation (state+PKCE+code) and eligibility (email-domain +
// user-cap) live in shared helpers (validateOAuthCallback, checkUserEligibility);
// the provider-specific code exchange and user creation stay here.
func HandleGoogleCallback(config *OAuthConfig, database *db.DB) http.HandlerFunc {
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

		// Exchange code for access token
		accessToken, err := exchangeGoogleCode(code, codeVerifier, config)
		if err != nil {
			log.Error("Failed to exchange Google code", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=google_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to complete Google authentication. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Get user info from Google
		user, err := getGoogleUser(accessToken)
		if err != nil {
			log.Error("Failed to get Google user", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=google_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to retrieve user information from Google. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// SECURITY: Reject unverified emails
		if !user.VerifiedEmail {
			log.Warn("Google email not verified", "email", user.Email)
			errorURL := fmt.Sprintf("%s/login?error=email_unverified&error_description=%s",
				frontendURL,
				url.QueryEscape("Your Google email is not verified. Please verify your email and try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		log.Info("Google OAuth user retrieved",
			"google_id", user.ID,
			"email", user.Email,
			"name", user.Name)

		// CF-483: never let the demo email log in via OAuth.
		if IsDemoLoginEmail(config.DemoIdentityEmail, user.Email) {
			log.Warn("Google OAuth login attempt for demo identity rejected", "email", user.Email)
			redirectDemoLoginRejected(w, r, frontendURL)
			return
		}

		// Check email domain restriction + user cap (shared across providers).
		if err := checkUserEligibility(ctx, database, user.Email, config.AllowedEmailDomains); err != nil {
			redirectUserIneligible(w, r, frontendURL, "google", user.Email, err)
			return
		}

		// Find or create user in database
		oauthInfo := models.OAuthUserInfo{
			Provider:   models.ProviderGoogle,
			ProviderID: user.ID,
			Email:      user.Email,
			Name:       user.Name,
			AvatarURL:  user.Picture,
		}
		dbUser, err := authStore.FindOrCreateUserByOAuth(ctx, oauthInfo, config.AutoLinkEmail)
		if err != nil {
			if errors.Is(err, db.ErrAutoLinkDisabled) {
				log.Warn("OAuth auto-link disabled; refusing to link to existing account", "email", oauthInfo.Email, "provider", "google")
				errorURL := fmt.Sprintf("%s/login?error=account_exists&error_description=%s",
					frontendURL,
					url.QueryEscape("An account with this email already exists. Sign in with your original method."))
				http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
				return
			}
			log.Error("Failed to create/find user in database", "error", err, "google_id", user.ID)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// w8tz: reject deactivated accounts BEFORE minting a session, so the
		// login loop (app→401→login→app) can never start for an inactive user.
		if dbUser.Status == models.UserStatusInactive {
			log.Warn("OAuth login blocked for inactive user", "email", user.Email, "provider", "google")
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
		expectedEmail, emailMismatch := checkExpectedEmailMismatch(w, r, user.Email, "google")
		handlePostLoginRedirect(w, r, frontendURL, user.Email, expectedEmail, emailMismatch)
	}
}

// exchangeGoogleCode exchanges authorization code for access token
func exchangeGoogleCode(code, codeVerifier string, config *OAuthConfig) (string, error) {
	data := url.Values{
		"client_id":     {config.GoogleClientID},
		"client_secret": {config.GoogleClientSecret},
		"code":          {code},
		"redirect_uri":  {config.GoogleRedirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier}, // PKCE (r9zn)
	}

	resp, err := oauthHTTPClient().PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading Google token response: %w", err)
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
		return "", fmt.Errorf("google oauth error: %s - %s", result.Error, result.ErrorDesc)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return result.AccessToken, nil
}

// getGoogleUser fetches user info from Google
func getGoogleUser(accessToken string) (*googleUser, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user googleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// Normalize email to lowercase - emails are case-insensitive by convention (RFC 5321)
	user.Email = strings.ToLower(user.Email)

	// Validate email format
	if !validation.IsValidEmail(user.Email) {
		return nil, fmt.Errorf("invalid email format from Google: %q", user.Email)
	}

	return &user, nil
}

// ============================================================================
// Generic OIDC (Okta, Auth0, Azure AD, Keycloak, etc.)
// ============================================================================
