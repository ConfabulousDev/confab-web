package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

const (
	SessionCookieName = "confab_session"
	SessionDuration   = 7 * 24 * time.Hour // 7 days
	// OAuthAPITimeout is the timeout for GitHub OAuth API calls
	// Protects against hanging indefinitely if GitHub API is slow/unresponsive
	OAuthAPITimeout = 30 * time.Second
)

// cookieSecure returns whether cookies should have Secure flag
// Secure by default (HTTPS only), can be disabled for local dev
func cookieSecure() bool {
	// Only disable in local development - name is intentionally scary
	return os.Getenv("INSECURE_DEV_MODE") != "true"
}

// oauthHTTPClient returns an HTTP client with timeout for OAuth API calls
func oauthHTTPClient() *http.Client {
	return &http.Client{
		Timeout: OAuthAPITimeout,
	}
}

// OAuthConfig holds OAuth configuration for all providers
type OAuthConfig struct {
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

// GitHubUser represents GitHub user info from OAuth
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubEmail represents email from GitHub API
type GitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// HandleGitHubLogin initiates GitHub OAuth flow
func HandleGitHubLogin(config OAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate random state for CSRF protection
		state, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}

		// Store state in cookie for validation
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			MaxAge:   300, // 5 minutes
			HttpOnly: true,
			Secure:   cookieSecure(), // HTTPS-only (set INSECURE_DEV_MODE=true to disable for local dev)
			SameSite: http.SameSiteLaxMode,
		})

		// Store post-login redirect URL if provided (e.g., from /device page)
		if redirectAfter := r.URL.Query().Get("redirect"); redirectAfter != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     "post_login_redirect",
				Value:    redirectAfter,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(),
				SameSite: http.SameSiteLaxMode,
			})
		}

		// Store expected email if provided (for share link login flow)
		// Only store and use valid email addresses
		expectedEmail := r.URL.Query().Get("email")
		validEmail := expectedEmail != "" && validation.IsValidEmail(expectedEmail)
		if validEmail {
			http.SetCookie(w, &http.Cookie{
				Name:     "expected_email",
				Value:    expectedEmail,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(),
				SameSite: http.SameSiteLaxMode,
			})
		}

		// Redirect to GitHub
		// Scope: read:user gets profile info, user:email gets email
		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=read:user user:email",
			config.GitHubClientID,
			config.GitHubRedirectURL,
			state,
		)

		// Add login hint if valid email is provided (pre-fills GitHub username/email field)
		if validEmail {
			authURL += "&login=" + url.QueryEscape(expectedEmail)
		}

		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleGitHubCallback handles the OAuth callback from GitHub
func HandleGitHubCallback(config OAuthConfig, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Validate state to prevent CSRF
		stateCookie, err := r.Cookie("oauth_state")
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Clear state cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "oauth_state",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code parameter", http.StatusBadRequest)
			return
		}

		// Exchange code for access token
		accessToken, err := exchangeGitHubCode(code, config)
		if err != nil {
			logger.Error("Failed to exchange GitHub code", "error", err)
			frontendURL := os.Getenv("FRONTEND_URL")
			errorURL := fmt.Sprintf("%s?error=github_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to complete GitHub authentication. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Get user info from GitHub
		user, err := getGitHubUser(accessToken)
		if err != nil {
			logger.Error("Failed to get GitHub user", "error", err)
			frontendURL := os.Getenv("FRONTEND_URL")
			errorURL := fmt.Sprintf("%s?error=github_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to retrieve user information from GitHub. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Log user info from GitHub
		logger.Info("GitHub OAuth user retrieved",
			"github_id", user.ID,
			"login", user.Login,
			"email", user.Email,
			"name", user.Name)

		// Check user cap
		allowed, err := CanUserLogin(ctx, database, user.Email)
		if err != nil {
			logger.Error("Failed to check user login eligibility", "error", err, "email", user.Email)
			frontendURL := os.Getenv("FRONTEND_URL")
			errorURL := fmt.Sprintf("%s?error=server_error&error_description=%s",
				frontendURL,
				url.QueryEscape("An error occurred. Please try again later."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}
		if !allowed {
			logger.Warn("User cap reached, login denied", "email", user.Email)
			frontendURL := os.Getenv("FRONTEND_URL")
			errorURL := fmt.Sprintf("%s?error=access_denied&error_description=%s",
				frontendURL,
				url.QueryEscape("This application has reached its user limit. Please contact the administrator."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
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
		dbUser, err := database.FindOrCreateUserByOAuth(ctx, oauthInfo)
		if err != nil {
			logger.Error("Failed to create/find user in database", "error", err, "github_id", oauthInfo.ProviderID)
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
		if err := database.CreateWebSession(ctx, sessionID, dbUser.ID, expiresAt); err != nil {
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

		// Check for expected email (from share link flow)
		// If user logged in with different email than expected, redirect with mismatch error
		var emailMismatch bool
		var expectedEmail string
		if expectedEmailCookie, err := r.Cookie("expected_email"); err == nil && expectedEmailCookie.Value != "" {
			expectedEmail = expectedEmailCookie.Value
			// Clear the cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "expected_email",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			// Compare emails (case-insensitive)
			if !strings.EqualFold(expectedEmail, user.Email) {
				emailMismatch = true
				logger.Warn("OAuth email mismatch",
					"expected_email", expectedEmail,
					"actual_email", user.Email,
					"provider", "github")
			}
		}

		// Check if this was a CLI login flow
		if cliRedirect, err := r.Cookie("cli_redirect"); err == nil && cliRedirect.Value != "" {
			// Clear the cli_redirect cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "cli_redirect",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			// Redirect back to CLI authorize endpoint
			http.Redirect(w, r, cliRedirect.Value, http.StatusTemporaryRedirect)
			return
		}

		// Check if there's a post-login redirect (e.g., from /device page or protected frontend route)
		if postLoginRedirect, err := r.Cookie("post_login_redirect"); err == nil && postLoginRedirect.Value != "" {
			// Clear the cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "post_login_redirect",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			redirectURL := postLoginRedirect.Value
			// SECURITY: Only allow relative paths to prevent open redirect attacks
			if !strings.HasPrefix(redirectURL, "/") || strings.HasPrefix(redirectURL, "//") {
				logger.Warn("Blocked potential open redirect", "redirect_url", redirectURL)
				redirectURL = "/"
			}
			// If it's a frontend path (not a backend path like /device), prepend frontend URL
			if !strings.HasPrefix(redirectURL, "/auth") && !strings.HasPrefix(redirectURL, "/device") {
				redirectURL = os.Getenv("FRONTEND_URL") + redirectURL
			}
			// Add email mismatch params if applicable
			if emailMismatch {
				separator := "?"
				if strings.Contains(redirectURL, "?") {
					separator = "&"
				}
				redirectURL += separator + "email_mismatch=1&expected=" + url.QueryEscape(expectedEmail) + "&actual=" + url.QueryEscape(user.Email)
			}
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			return
		}

		// Redirect back to frontend (normal web login)
		// Note: FRONTEND_URL is validated at startup in main.go
		frontendURL := os.Getenv("FRONTEND_URL")
		if emailMismatch {
			frontendURL += "?email_mismatch=1&expected=" + url.QueryEscape(expectedEmail) + "&actual=" + url.QueryEscape(user.Email)
		}
		http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
	}
}

// HandleLogout logs out the user
func HandleLogout(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get session cookie
		cookie, err := r.Cookie(SessionCookieName)
		if err == nil {
			// Delete session from database
			database.DeleteWebSession(ctx, cookie.Value)
		}

		// Clear session cookie
		http.SetCookie(w, &http.Cookie{
			Name:   SessionCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		// Check for redirect URL (e.g., for re-login with different account)
		frontendURL := os.Getenv("FRONTEND_URL")
		if redirectAfter := r.URL.Query().Get("redirect"); redirectAfter != "" {
			// SECURITY: Only allow relative paths to prevent open redirect attacks
			if strings.HasPrefix(redirectAfter, "/") && !strings.HasPrefix(redirectAfter, "//") {
				// Prepend frontend URL for frontend paths, or use as-is for backend paths
				if strings.HasPrefix(redirectAfter, "/auth") {
					http.Redirect(w, r, redirectAfter, http.StatusTemporaryRedirect)
					return
				}
				http.Redirect(w, r, frontendURL+redirectAfter, http.StatusTemporaryRedirect)
				return
			}
			logger.Warn("Blocked potential open redirect in logout", "redirect_url", redirectAfter)
		}

		// Redirect back to frontend
		// Note: FRONTEND_URL is validated at startup in main.go
		http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
	}
}

// TrySessionAuth attempts to authenticate using a session cookie.
// Returns the user ID if successful, nil otherwise.
// Does not reject - callers decide whether to require auth.
func TrySessionAuth(r *http.Request, database *db.DB) *int64 {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil
	}

	session, err := database.GetWebSession(r.Context(), cookie.Value)
	if err != nil {
		return nil
	}

	// Check if user is inactive
	if session.UserStatus == models.UserStatusInactive {
		return nil
	}

	return &session.UserID
}

// RequireSession returns an HTTP middleware that requires session cookie authentication.
// Use TrySessionAuth for optional authentication.
func RequireSession(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := TrySessionAuth(r, database)
			if userID == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set user ID on logger's response writer
			setLogUserID(w, *userID)

			// Add user ID to context
			ctx := context.WithValue(r.Context(), userIDContextKey, *userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSessionOrAPIKey returns an HTTP middleware that requires either
// session cookie or API key authentication. Tries session first, then API key.
func RequireSessionOrAPIKey(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try session cookie first
			userID := TrySessionAuth(r, database)

			// Fall back to API key
			if userID == nil {
				userID = TryAPIKeyAuth(r, database)
			}

			if userID == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set user ID on logger's response writer
			setLogUserID(w, *userID)

			// Add user ID to context
			ctx := context.WithValue(r.Context(), userIDContextKey, *userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth returns an HTTP middleware that attempts authentication but doesn't require it.
// If authentication succeeds (via session cookie or API key), the user ID is set in context.
// If authentication fails, the request continues without a user ID.
// Use auth.GetUserID(ctx) to check if a user is authenticated.
func OptionalAuth(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try API key first, then session cookie
			if userID := TryAPIKeyAuth(r, database); userID != nil {
				setLogUserID(w, *userID)
				ctx := context.WithValue(r.Context(), userIDContextKey, *userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if userID := TrySessionAuth(r, database); userID != nil {
				setLogUserID(w, *userID)
				ctx := context.WithValue(r.Context(), userIDContextKey, *userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No auth - continue without user ID in context
			next.ServeHTTP(w, r)
		})
	}
}

// exchangeGitHubCode exchanges authorization code for access token
func exchangeGitHubCode(code string, config OAuthConfig) (string, error) {
	data := map[string]string{
		"client_id":     config.GitHubClientID,
		"client_secret": config.GitHubClientSecret,
		"code":          code,
		"redirect_uri":  config.GitHubRedirectURL,
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")

	// Build query string
	q := req.URL.Query()
	for k, v := range data {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := oauthHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no access token in response")
	}

	return token, nil
}

// getGitHubUser fetches user info from GitHub
func getGitHubUser(accessToken string) (*GitHubUser, error) {
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

	var user GitHubUser
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

	var emails []GitHubEmail
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

// GoogleUser represents Google user info from OAuth
type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// HandleGoogleLogin initiates Google OAuth flow
func HandleGoogleLogin(config OAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate random state for CSRF protection
		state, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}

		// Store state in cookie for validation
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			MaxAge:   300, // 5 minutes
			HttpOnly: true,
			Secure:   cookieSecure(),
			SameSite: http.SameSiteLaxMode,
		})

		// Store post-login redirect URL if provided (e.g., from /device page)
		if redirectAfter := r.URL.Query().Get("redirect"); redirectAfter != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     "post_login_redirect",
				Value:    redirectAfter,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(),
				SameSite: http.SameSiteLaxMode,
			})
		}

		// Store expected email if provided (for share link login flow)
		// Only store and use valid email addresses
		expectedEmail := r.URL.Query().Get("email")
		validEmail := expectedEmail != "" && validation.IsValidEmail(expectedEmail)
		if validEmail {
			http.SetCookie(w, &http.Cookie{
				Name:     "expected_email",
				Value:    expectedEmail,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(),
				SameSite: http.SameSiteLaxMode,
			})
		}

		// Redirect to Google
		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=%s",
			url.QueryEscape(config.GoogleClientID),
			url.QueryEscape(config.GoogleRedirectURL),
			url.QueryEscape(state),
			url.QueryEscape("openid email profile"),
		)

		// Add login hint if valid email is provided (pre-fills Google email field)
		if validEmail {
			authURL += "&login_hint=" + url.QueryEscape(expectedEmail)
		}

		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleGoogleCallback handles the OAuth callback from Google
func HandleGoogleCallback(config OAuthConfig, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		frontendURL := os.Getenv("FRONTEND_URL")

		// Validate state to prevent CSRF
		stateCookie, err := r.Cookie("oauth_state")
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Clear state cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "oauth_state",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code parameter", http.StatusBadRequest)
			return
		}

		// Exchange code for access token
		accessToken, err := exchangeGoogleCode(code, config)
		if err != nil {
			logger.Error("Failed to exchange Google code", "error", err)
			errorURL := fmt.Sprintf("%s?error=google_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to complete Google authentication. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// Get user info from Google
		user, err := getGoogleUser(accessToken)
		if err != nil {
			logger.Error("Failed to get Google user", "error", err)
			errorURL := fmt.Sprintf("%s?error=google_error&error_description=%s",
				frontendURL,
				url.QueryEscape("Failed to retrieve user information from Google. Please try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		// SECURITY: Reject unverified emails
		if !user.VerifiedEmail {
			logger.Warn("Google email not verified", "email", user.Email)
			errorURL := fmt.Sprintf("%s?error=email_unverified&error_description=%s",
				frontendURL,
				url.QueryEscape("Your Google email is not verified. Please verify your email and try again."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}

		logger.Info("Google OAuth user retrieved",
			"google_id", user.ID,
			"email", user.Email,
			"name", user.Name)

		// Check user cap
		allowed, err := CanUserLogin(ctx, database, user.Email)
		if err != nil {
			logger.Error("Failed to check user login eligibility", "error", err, "email", user.Email)
			errorURL := fmt.Sprintf("%s?error=server_error&error_description=%s",
				frontendURL,
				url.QueryEscape("An error occurred. Please try again later."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
			return
		}
		if !allowed {
			logger.Warn("User cap reached, login denied", "email", user.Email)
			errorURL := fmt.Sprintf("%s?error=access_denied&error_description=%s",
				frontendURL,
				url.QueryEscape("This application has reached its user limit. Please contact the administrator."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
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
		dbUser, err := database.FindOrCreateUserByOAuth(ctx, oauthInfo)
		if err != nil {
			logger.Error("Failed to create/find user in database", "error", err, "google_id", user.ID)
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
		if err := database.CreateWebSession(ctx, sessionID, dbUser.ID, expiresAt); err != nil {
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

		// Check for expected email (from share link flow)
		// If user logged in with different email than expected, redirect with mismatch error
		var emailMismatch bool
		var expectedEmail string
		if expectedEmailCookie, err := r.Cookie("expected_email"); err == nil && expectedEmailCookie.Value != "" {
			expectedEmail = expectedEmailCookie.Value
			// Clear the cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "expected_email",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			// Compare emails (case-insensitive)
			if !strings.EqualFold(expectedEmail, user.Email) {
				emailMismatch = true
				logger.Warn("OAuth email mismatch",
					"expected_email", expectedEmail,
					"actual_email", user.Email,
					"provider", "google")
			}
		}

		// Check if this was a CLI login flow
		if cliRedirect, err := r.Cookie("cli_redirect"); err == nil && cliRedirect.Value != "" {
			http.SetCookie(w, &http.Cookie{
				Name:   "cli_redirect",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, cliRedirect.Value, http.StatusTemporaryRedirect)
			return
		}

		// Check if there's a post-login redirect (e.g., from /device page or protected frontend route)
		if postLoginRedirect, err := r.Cookie("post_login_redirect"); err == nil && postLoginRedirect.Value != "" {
			// Clear the cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "post_login_redirect",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			redirectURL := postLoginRedirect.Value
			// SECURITY: Only allow relative paths to prevent open redirect attacks
			if !strings.HasPrefix(redirectURL, "/") || strings.HasPrefix(redirectURL, "//") {
				logger.Warn("Blocked potential open redirect", "redirect_url", redirectURL)
				redirectURL = "/"
			}
			// If it's a frontend path (not a backend path like /device), prepend frontend URL
			if !strings.HasPrefix(redirectURL, "/auth") && !strings.HasPrefix(redirectURL, "/device") {
				redirectURL = frontendURL + redirectURL
			}
			// Add email mismatch params if applicable
			if emailMismatch {
				separator := "?"
				if strings.Contains(redirectURL, "?") {
					separator = "&"
				}
				redirectURL += separator + "email_mismatch=1&expected=" + url.QueryEscape(expectedEmail) + "&actual=" + url.QueryEscape(user.Email)
			}
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			return
		}

		// Redirect to frontend
		finalURL := frontendURL
		if emailMismatch {
			finalURL += "?email_mismatch=1&expected=" + url.QueryEscape(expectedEmail) + "&actual=" + url.QueryEscape(user.Email)
		}
		http.Redirect(w, r, finalURL, http.StatusTemporaryRedirect)
	}
}

// exchangeGoogleCode exchanges authorization code for access token
func exchangeGoogleCode(code string, config OAuthConfig) (string, error) {
	data := url.Values{
		"client_id":     {config.GoogleClientID},
		"client_secret": {config.GoogleClientSecret},
		"code":          {code},
		"redirect_uri":  {config.GoogleRedirectURL},
		"grant_type":    {"authorization_code"},
	}

	resp, err := oauthHTTPClient().PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

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
func getGoogleUser(accessToken string) (*GoogleUser, error) {
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

	var user GoogleUser
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

// generateRandomString generates a random string for sessions/state
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// HandleLoginSelector serves a page where users can choose their OAuth provider
func HandleLoginSelector() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Preserve any redirect URL and email in query params
		redirectAfter := r.URL.Query().Get("redirect")
		expectedEmail := r.URL.Query().Get("email")

		// Only use email if it's a valid format (prevents display of garbage/attacks)
		validEmail := expectedEmail != "" && validation.IsValidEmail(expectedEmail)
		if !validEmail {
			expectedEmail = ""
		}

		// Build query string for OAuth links
		buildQueryString := func() string {
			params := make([]string, 0, 2)
			if redirectAfter != "" {
				params = append(params, "redirect="+url.QueryEscape(redirectAfter))
			}
			if expectedEmail != "" {
				params = append(params, "email="+url.QueryEscape(expectedEmail))
			}
			if len(params) > 0 {
				return "?" + strings.Join(params, "&")
			}
			return ""
		}

		// Build subtitle based on whether expected email is set
		subtitle := "Choose your authentication method"
		if expectedEmail != "" {
			subtitle = fmt.Sprintf("Sign in with <strong>%s</strong> to view this shared session", template.HTMLEscapeString(expectedEmail))
		}

		queryString := buildQueryString()

		html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign In - Confab</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            background: #fff;
            padding: 2.5rem;
            border-radius: 6px;
            border: 1px solid #e5e5e5;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            text-align: center;
            max-width: 400px;
            width: 90%;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.25rem;
            font-weight: 600;
            color: #1a1a1a;
        }
        p {
            color: #666;
            margin: 0 0 1.5rem 0;
            font-size: 0.875rem;
        }
        p strong {
            color: #1a1a1a;
        }
        .buttons {
            display: flex;
            flex-direction: column;
            gap: 0.75rem;
        }
        .btn {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 0.5rem;
            padding: 0.625rem 1rem;
            border: 1px solid #e5e5e5;
            border-radius: 4px;
            font-size: 0.875rem;
            font-weight: 500;
            cursor: pointer;
            text-decoration: none;
            transition: all 0.15s ease;
        }
        .btn:hover {
            border-color: #ccc;
        }
        .btn-github {
            background: #24292e;
            color: #fff;
            border-color: #24292e;
        }
        .btn-github:hover {
            background: #1b1f23;
            border-color: #1b1f23;
        }
        .btn-google {
            background: #fff;
            color: #1a1a1a;
        }
        .btn-google:hover {
            background: #f5f5f5;
        }
        .icon {
            width: 18px;
            height: 18px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Sign in to Confab</h1>
        <p>` + subtitle + `</p>
        <div class="buttons">
            <a href="/auth/github/login` + queryString + `" class="btn btn-github">
                <svg class="icon" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
                </svg>
                Continue with GitHub
            </a>
            <a href="/auth/google/login` + queryString + `" class="btn btn-google">
                <svg class="icon" viewBox="0 0 24 24">
                    <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                    <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                    <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                    <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
                Continue with Google
            </a>
        </div>
    </div>
</body>
</html>`

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// HandleCLIAuthorize handles CLI API key generation flow
func HandleCLIAuthorize(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check if user confirmed provider choice (came back from login selector)
		confirmed := r.URL.Query().Get("confirmed") == "1"

		// Get session cookie (user must be logged in via web)
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			// No session - redirect to login selector, then back here
			redirectURL := "/auth/cli/authorize?" + r.URL.RawQuery + "&confirmed=1"
			http.SetCookie(w, &http.Cookie{
				Name:     "cli_redirect",
				Value:    redirectURL,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(), // HTTPS-only (set INSECURE_DEV_MODE=true to disable for local dev)
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
			return
		}

		// Validate session
		session, err := database.GetWebSession(ctx, cookie.Value)
		if err != nil {
			// Session is invalid or expired - clear the stale cookie and redirect to login
			http.SetCookie(w, &http.Cookie{
				Name:   SessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})

			// Redirect to login selector, then back here
			redirectURL := "/auth/cli/authorize?" + r.URL.RawQuery + "&confirmed=1"
			http.SetCookie(w, &http.Cookie{
				Name:     "cli_redirect",
				Value:    redirectURL,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(), // HTTPS-only (set INSECURE_DEV_MODE=true to disable for local dev)
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
			return
		}

		// If user has session but hasn't confirmed provider choice yet, show selector
		// This allows users to switch accounts/providers during CLI login
		if !confirmed {
			redirectURL := "/auth/cli/authorize?" + r.URL.RawQuery + "&confirmed=1"
			http.SetCookie(w, &http.Cookie{
				Name:     "cli_redirect",
				Value:    redirectURL,
				Path:     "/",
				MaxAge:   300, // 5 minutes
				HttpOnly: true,
				Secure:   cookieSecure(),
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
			return
		}

		// Get callback URL and key name from query params
		callback := r.URL.Query().Get("callback")
		keyName := r.URL.Query().Get("name")

		if callback == "" {
			http.Error(w, "Missing callback parameter", http.StatusBadRequest)
			return
		}

		if keyName == "" {
			keyName = "CLI Key"
		}

		// Validate callback is localhost
		if !isLocalhostURL(callback) {
			http.Error(w, "Callback must be localhost", http.StatusBadRequest)
			return
		}

		// Generate API key
		apiKey, keyHash, err := GenerateAPIKey()
		if err != nil {
			http.Error(w, "Failed to generate API key", http.StatusInternalServerError)
			return
		}

		// Store in database
		keyID, createdAt, err := database.CreateAPIKeyWithReturn(ctx, session.UserID, keyHash, keyName)
		if err != nil {
			if err == db.ErrAPIKeyLimitExceeded {
				// Redirect to callback with error that CLI can handle
				frontendURL := os.Getenv("FRONTEND_URL")
				redirectURL := fmt.Sprintf("%s?error=api_key_limit_exceeded", callback)
				logger.Warn("API key limit exceeded", "user_id", session.UserID)
				// Also show a helpful page before redirecting
				html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="refresh" content="5;url=%s">
    <title>API Key Limit Reached - Confab</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #fafafa; color: #1a1a1a; }
        .container { background: #fff; padding: 2.5rem; border-radius: 6px; border: 1px solid #e5e5e5; box-shadow: 0 1px 3px rgba(0,0,0,0.08); text-align: center; max-width: 500px; }
        h1 { color: #dc2626; font-size: 1.25rem; font-weight: 600; margin-bottom: 0.75rem; }
        p { color: #666; font-size: 0.875rem; margin-bottom: 1rem; }
        a { color: #0066cc; }
    </style>
</head>
<body>
    <div class="container">
        <h1>API Key Limit Reached</h1>
        <p>You have reached the maximum of 100 API keys. Please delete some unused keys before creating new ones.</p>
        <p><a href="%s/settings/api-keys">Manage your API keys</a></p>
        <p style="font-size: 0.75rem; color: #999;">Redirecting to CLI in 5 seconds...</p>
    </div>
</body>
</html>`, redirectURL, frontendURL)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(html))
				return
			}
			logger.Error("Failed to create API key in database", "error", err, "user_id", session.UserID)
			http.Error(w, "Failed to create API key", http.StatusInternalServerError)
			return
		}

		logger.Info("API key created successfully",
			"key_id", keyID,
			"name", keyName,
			"user_id", session.UserID,
			"created_at", createdAt)

		// Redirect to callback with API key
		redirectURL := fmt.Sprintf("%s?key=%s", callback, apiKey)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

// ============================================================================
// Device Code Flow (for CLI authentication without browser on same machine)
// ============================================================================

const (
	// DeviceCodeExpiry is how long a device code is valid
	DeviceCodeExpiry = 5 * time.Minute
	// DeviceCodePollInterval is the minimum interval between poll requests
	DeviceCodePollInterval = 5 * time.Second
)

// DeviceCodeRequest is the request body for /auth/device/code
type DeviceCodeRequest struct {
	KeyName string `json:"key_name"`
}

// DeviceCodeResponse is the response from /auth/device/code
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`       // seconds
	Interval        int    `json:"interval"`         // polling interval in seconds
}

// DeviceTokenRequest is the request body for /auth/device/token
type DeviceTokenRequest struct {
	DeviceCode string `json:"device_code"`
}

// DeviceTokenResponse is the response from /auth/device/token
type DeviceTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Error       string `json:"error,omitempty"`
}

// generateUserCode generates a human-friendly code (e.g., "ABCD-1234")
func generateUserCode() (string, error) {
	// Use uppercase letters (excluding confusing ones: 0, O, I, L, 1)
	const chars = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	code := make([]byte, 8)
	for i := range code {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		code[i] = chars[int(b[0])%len(chars)]
	}
	// Format as XXXX-XXXX
	return string(code[:4]) + "-" + string(code[4:]), nil
}

// generateDeviceCode generates a secure random device code
func generateDeviceCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}

// HandleDeviceCode initiates a device code flow
// POST /auth/device/code
func HandleDeviceCode(database *db.DB, backendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse request
		var req DeviceCodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Allow empty body, use default key name
			req.KeyName = "CLI Key"
		}
		if req.KeyName == "" {
			req.KeyName = "CLI Key"
		}

		// Validate key name length
		if err := validation.ValidateAPIKeyName(req.KeyName); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate codes
		deviceCode, err := generateDeviceCode()
		if err != nil {
			logger.Error("Failed to generate device code", "error", err)
			http.Error(w, "Failed to generate device code", http.StatusInternalServerError)
			return
		}

		userCode, err := generateUserCode()
		if err != nil {
			logger.Error("Failed to generate user code", "error", err)
			http.Error(w, "Failed to generate user code", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(DeviceCodeExpiry)

		// Store in database
		if err := database.CreateDeviceCode(ctx, deviceCode, userCode, req.KeyName, expiresAt); err != nil {
			logger.Error("Failed to store device code", "error", err)
			http.Error(w, "Failed to create device code", http.StatusInternalServerError)
			return
		}

		logger.Info("Device code created", "user_code", userCode)

		// Return response
		resp := DeviceCodeResponse{
			DeviceCode:      deviceCode,
			UserCode:        userCode,
			VerificationURI: backendURL + "/auth/device",
			ExpiresIn:       int(DeviceCodeExpiry.Seconds()),
			Interval:        int(DeviceCodePollInterval.Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleDeviceToken exchanges a device code for an API key
// POST /auth/device/token
func HandleDeviceToken(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse request
		var req DeviceTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "invalid_request"})
			return
		}

		if req.DeviceCode == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "invalid_request"})
			return
		}

		// Look up device code
		dc, err := database.GetDeviceCodeByDeviceCode(ctx, req.DeviceCode)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if err == db.ErrDeviceCodeNotFound {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "invalid_grant"})
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "server_error"})
			}
			return
		}

		// Check if expired
		if time.Now().UTC().After(dc.ExpiresAt) {
			database.DeleteDeviceCode(ctx, req.DeviceCode)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "expired_token"})
			return
		}

		// Check if authorized
		if dc.AuthorizedAt == nil || dc.UserID == nil {
			// Not yet authorized - tell client to keep polling
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "authorization_pending"})
			return
		}

		// Authorized! Generate API key
		apiKey, keyHash, err := GenerateAPIKey()
		if err != nil {
			logger.Error("Failed to generate API key", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "server_error"})
			return
		}

		// Store API key
		keyID, createdAt, err := database.CreateAPIKeyWithReturn(ctx, *dc.UserID, keyHash, dc.KeyName)
		if err != nil {
			if err == db.ErrAPIKeyLimitExceeded {
				logger.Warn("API key limit exceeded during device flow", "user_id", *dc.UserID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "api_key_limit_exceeded"})
				return
			}
			logger.Error("Failed to create API key", "error", err, "user_id", *dc.UserID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(DeviceTokenResponse{Error: "server_error"})
			return
		}

		logger.Info("API key created via device flow",
			"key_id", keyID,
			"name", dc.KeyName,
			"user_id", *dc.UserID,
			"created_at", createdAt)

		// Delete the device code (one-time use)
		database.DeleteDeviceCode(ctx, req.DeviceCode)

		// Return the API key
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceTokenResponse{
			AccessToken: apiKey,
			TokenType:   "Bearer",
		})
	}
}

// HandleDevicePage serves the device verification page
// GET /auth/device
func HandleDevicePage(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get pre-filled code from query param
		prefilledCode := r.URL.Query().Get("code")

		// Check if user is logged in
		cookie, err := r.Cookie(SessionCookieName)
		loggedIn := err == nil && cookie.Value != ""

		if loggedIn {
			_, err := database.GetWebSession(r.Context(), cookie.Value)
			if err != nil {
				loggedIn = false
			}
		}

		// If not logged in, redirect directly to login selector
		if !loggedIn {
			redirectURL := "/auth/device"
			if prefilledCode != "" {
				redirectURL = "/auth/device?code=" + url.QueryEscape(prefilledCode)
			}
			loginURL := "/auth/login?redirect=" + url.QueryEscape(redirectURL)
			http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
			return
		}

		// Logged in - show the code entry form
		html := generateDevicePageHTML(prefilledCode)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// HandleDeviceVerify handles the form submission to verify a device code
// POST /device/verify
func HandleDeviceVerify(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse form first to get the code for redirect
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		userCode := strings.ToUpper(strings.TrimSpace(r.FormValue("code")))

		// Build redirect URL with code preserved
		redirectURL := "/auth/device"
		if userCode != "" {
			redirectURL = "/auth/device?code=" + url.QueryEscape(userCode)
		}
		loginRedirect := "/auth/login?redirect=" + url.QueryEscape(redirectURL)

		// Must be logged in
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Redirect(w, r, loginRedirect, http.StatusTemporaryRedirect)
			return
		}

		session, err := database.GetWebSession(ctx, cookie.Value)
		if err != nil {
			http.Redirect(w, r, loginRedirect, http.StatusTemporaryRedirect)
			return
		}

		// Normalize: remove any dashes and re-add in correct position
		userCode = strings.ReplaceAll(userCode, "-", "")
		if len(userCode) == 8 {
			userCode = userCode[:4] + "-" + userCode[4:]
		}

		// Validate and authorize
		err = database.AuthorizeDeviceCode(ctx, userCode, session.UserID)
		if err != nil {
			logger.Warn("Device code authorization failed", "error", err, "user_code", userCode)
			// Show error page
			html := generateDeviceResultHTML(false, "Invalid or expired code. Please try again.")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(html))
			return
		}

		logger.Info("Device code authorized", "user_code", userCode, "user_id", session.UserID)

		// Show success page
		html := generateDeviceResultHTML(true, "Device authorized! You can close this window and return to your terminal.")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

func generateDevicePageHTML(prefilledCode string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Authorize Device - Confab</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            background: #fff;
            padding: 2.5rem;
            border-radius: 6px;
            border: 1px solid #e5e5e5;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            text-align: center;
            max-width: 400px;
            width: 90%%;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.25rem;
            font-weight: 600;
            color: #1a1a1a;
        }
        p {
            color: #666;
            margin: 0 0 1.5rem 0;
            font-size: 0.875rem;
        }
        form {
            display: flex;
            flex-direction: column;
            gap: 0.75rem;
        }
        input[type="text"] {
            padding: 0.75rem;
            font-size: 1.25rem;
            text-align: center;
            letter-spacing: 0.2em;
            text-transform: uppercase;
            border: 1px solid #e5e5e5;
            border-radius: 4px;
            background: #fff;
            color: #1a1a1a;
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, monospace;
        }
        input[type="text"]:focus {
            outline: none;
            border-color: #0066cc;
        }
        button {
            padding: 0.625rem 1rem;
            font-size: 0.875rem;
            font-weight: 500;
            border: none;
            border-radius: 4px;
            background: #0066cc;
            color: #fff;
            cursor: pointer;
            transition: background 0.15s ease;
        }
        button:hover {
            background: #0052a3;
        }
        .hint {
            font-size: 0.75rem;
            color: #999;
            margin-top: 1rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Authorize Device</h1>
        <p>Enter the code shown in your terminal to connect your CLI.</p>
        <form action="/auth/device/verify" method="POST">
            <input type="text" name="code" placeholder="XXXX-XXXX" maxlength="9"
                   value="%s" autocomplete="off" autofocus>
            <button type="submit">Authorize</button>
        </form>
        <p class="hint">The code expires in 15 minutes.</p>
    </div>
</body>
</html>`, prefilledCode)
}

func generateDeviceResultHTML(success bool, message string) string {
	icon := "✗"
	iconColor := "#dc2626"
	if success {
		icon = "✓"
		iconColor = "#16a34a"
	}

	// Add link to frontend on success
	homeLink := ""
	if success {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL != "" {
			homeLink = fmt.Sprintf(`<a href="%s" class="home-link">Go to Confab</a>`, frontendURL)
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Device Authorization - Confab</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #fafafa;
            color: #1a1a1a;
        }
        .container {
            background: #fff;
            padding: 2.5rem;
            border-radius: 6px;
            border: 1px solid #e5e5e5;
            box-shadow: 0 1px 3px rgba(0,0,0,0.08);
            text-align: center;
            max-width: 400px;
            width: 90%%;
        }
        .icon {
            font-size: 3rem;
            color: %s;
            margin-bottom: 0.75rem;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.25rem;
            font-weight: 600;
            color: #1a1a1a;
        }
        p {
            color: #666;
            margin: 0;
            font-size: 0.875rem;
        }
        .home-link {
            display: inline-block;
            margin-top: 1.5rem;
            padding: 0.625rem 1rem;
            background: #0066cc;
            color: #fff;
            text-decoration: none;
            border-radius: 4px;
            font-size: 0.875rem;
            font-weight: 500;
            transition: background 0.15s ease;
        }
        .home-link:hover {
            background: #0052a3;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">%s</div>
        <h1>%s</h1>
        <p>%s</p>
        %s
    </div>
</body>
</html>`, iconColor, icon, func() string {
		if success {
			return "Success!"
		}
		return "Error"
	}(), message, homeLink)
}

// isLocalhostURL checks if URL is localhost
// Properly validates URL to prevent open redirect attacks
func isLocalhostURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// Parse URL properly using net/url
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Only allow http scheme (localhost doesn't need https)
	if u.Scheme != "http" {
		return false
	}

	// Get hostname without port
	hostname := u.Hostname()

	// Only allow localhost or 127.0.0.1
	if hostname != "localhost" && hostname != "127.0.0.1" {
		return false
	}

	// Reject URLs with username/password (e.g., http://localhost@evil.com)
	if u.User != nil {
		return false
	}

	// Validate port if present (optional but good practice)
	if port := u.Port(); port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return false
		}
		// Port must be in valid range
		if portNum < 1 || portNum > 65535 {
			return false
		}
	}

	return true
}

// DefaultMaxUsers is the default maximum number of users allowed in the system
const DefaultMaxUsers = 50

// CanUserLogin checks if a user can log in based on the user cap
// Returns true if:
// 1. User already exists (returning users always allowed), OR
// 2. User count is below MAX_USERS cap (new users allowed if under cap)
func CanUserLogin(ctx context.Context, database *db.DB, email string) (bool, error) {
	if database == nil {
		return false, fmt.Errorf("database is required")
	}

	// Validate email format (also rejects empty and whitespace-only emails)
	if !validation.IsValidEmail(email) {
		return false, nil
	}

	// Check if user already exists - returning users always allowed
	exists, err := database.UserExistsByEmail(ctx, email)
	if err != nil {
		logger.Warn("Failed to check if user exists", "email", email, "error", err)
		return false, err
	}
	if exists {
		return true, nil
	}

	// New user - check the user cap
	maxUsers := DefaultMaxUsers
	if maxUsersEnv := os.Getenv("MAX_USERS"); maxUsersEnv != "" {
		parsed, err := strconv.Atoi(maxUsersEnv)
		if err != nil {
			logger.Warn("Invalid MAX_USERS value, using default", "value", maxUsersEnv, "default", DefaultMaxUsers, "error", err)
		} else {
			maxUsers = parsed
		}
	}

	currentUsers, err := database.CountUsers(ctx)
	if err != nil {
		logger.Warn("Failed to count users", "error", err)
		return false, err
	}

	if currentUsers >= maxUsers {
		logger.Warn("User cap reached", "current", currentUsers, "max", maxUsers, "email", email)
		return false, nil
	}

	return true, nil
}
