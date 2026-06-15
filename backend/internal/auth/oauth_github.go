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

// githubUser represents GitHub user info from OAuth
type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// githubEmail represents email from GitHub API
type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// HandleGitHubLogin initiates GitHub OAuth flow
func HandleGitHubLogin(config *OAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, challenge, validEmail, expectedEmail, err := setOAuthLoginCookies(w, r)
		if err != nil {
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}

		// Scope: read:user gets profile info, user:email gets email
		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=read:user user:email",
			config.GitHubClientID,
			config.GitHubRedirectURL,
			state,
		)

		// PKCE (S256): bind the auth code to this browser's verifier cookie (r9zn).
		authURL += "&code_challenge=" + url.QueryEscape(challenge) + "&code_challenge_method=S256"

		if validEmail {
			authURL += "&login=" + url.QueryEscape(expectedEmail)
		}

		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleGitHubCallback handles the OAuth callback from GitHub.
// Cross-provider validation (state+PKCE+code) and eligibility (email-domain +
// user-cap) live in shared helpers (validateOAuthCallback, checkUserEligibility);
// the provider-specific code exchange and user creation stay here.
func HandleGitHubCallback(config *OAuthConfig, database *db.DB) http.HandlerFunc {
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
		accessToken, err := exchangeGitHubCode(code, codeVerifier, config)
		if err != nil {
			log.Error("Failed to exchange GitHub code", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=github_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to complete GitHub authentication. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Get user info from GitHub
		user, err := getGitHubUser(accessToken)
		if err != nil {
			log.Error("Failed to get GitHub user", "error", err)
			errorURL := fmt.Sprintf("%s/login?error=github_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to retrieve user information from GitHub. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Log user info from GitHub
		log.Info("GitHub OAuth user retrieved",
			"github_id", user.ID,
			"login", user.Login,
			"email", user.Email,
			"name", user.Name)

		// CF-483: never let the demo email log in via OAuth — closes the
		// email-linking vector in FindOrCreateUserByOAuth.
		if IsDemoLoginEmail(config.DemoIdentityEmail, user.Email) {
			log.Warn("GitHub OAuth login attempt for demo identity rejected", "email", user.Email)
			redirectDemoLoginRejected(w, r, frontendURL)
			return
		}

		// Check email domain restriction + user cap (shared across providers).
		if err := checkUserEligibility(ctx, database, user.Email, config.AllowedEmailDomains); err != nil {
			redirectUserIneligible(w, r, frontendURL, "github", user.Email, err)
			return
		}

		// Use login (username) as fallback if name is empty
		displayName := user.Name
		if displayName == "" {
			displayName = user.Login
		}

		// Find or create user in database using generic OAuth function
		oauthInfo := models.OAuthUserInfo{
			Provider:         models.ProviderGitHub,
			ProviderID:       fmt.Sprintf("%d", user.ID),
			ProviderUsername: user.Login,
			Email:            user.Email,
			Name:             displayName,
			AvatarURL:        user.AvatarURL,
		}
		dbUser, err := authStore.FindOrCreateUserByOAuth(ctx, oauthInfo, config.AutoLinkEmail)
		if err != nil {
			if errors.Is(err, db.ErrAutoLinkDisabled) {
				log.Warn("OAuth auto-link disabled; refusing to link to existing account", "email", oauthInfo.Email, "provider", "github")
				errorURL := fmt.Sprintf("%s/login?error=account_exists&error_description=%s",
					frontendURL,
					url.QueryEscape("An account with this email already exists. Sign in with your original method."))
				http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
				return
			}
			log.Error("Failed to create/find user in database", "error", err, "github_id", oauthInfo.ProviderID)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
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
			Secure:   cookieSecure(), // HTTPS-only (set INSECURE_DEV_MODE=true to disable for local dev)
			SameSite: http.SameSiteLaxMode,
		})

		// Handle email mismatch check and post-login redirect
		expectedEmail, emailMismatch := checkExpectedEmailMismatch(w, r, user.Email, "github")
		handlePostLoginRedirect(w, r, frontendURL, user.Email, expectedEmail, emailMismatch)
	}
}

// exchangeGitHubCode exchanges authorization code for access token
func exchangeGitHubCode(code, codeVerifier string, config *OAuthConfig) (string, error) {
	data := url.Values{
		"client_id":     {config.GitHubClientID},
		"client_secret": {config.GitHubClientSecret},
		"code":          {code},
		"redirect_uri":  {config.GitHubRedirectURL},
		"code_verifier": {codeVerifier}, // PKCE (r9zn)
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token?"+data.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading GitHub token response: %w", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return result.AccessToken, nil
}

// getGitHubUser fetches user info from GitHub
func getGitHubUser(accessToken string) (*githubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// Always fetch verified email from GitHub
	// Don't trust the public email from /user endpoint (may not be verified)
	email, err := getGitHubPrimaryEmail(accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get verified email: %w", err)
	}
	// Normalize email to lowercase - emails are case-insensitive by convention (RFC 5321)
	email = strings.ToLower(email)

	// Validate email format
	if !validation.IsValidEmail(email) {
		return nil, fmt.Errorf("invalid email format from GitHub: %q", email)
	}
	user.Email = email

	return &user, nil
}

// getGitHubPrimaryEmail fetches primary email from GitHub
func getGitHubPrimaryEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// SECURITY: Only return PRIMARY + VERIFIED email
	// Never return unverified emails (user-controlled, not trustworthy)
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	// If no verified email, reject the login
	return "", fmt.Errorf("no verified email found - please verify your email on GitHub")
}

// ============================================================================
// Google OAuth
// ============================================================================
