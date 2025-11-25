package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
	"github.com/santaclaude2025/confab/backend/internal/models"
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

		// Redirect to GitHub
		// Scope: read:user gets profile info, user:email gets email
		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=read:user user:email",
			config.GitHubClientID,
			config.GitHubRedirectURL,
			state,
		)
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

		// Check email whitelist (if configured)
		if !isEmailAllowed(user.Email) {
			logger.Warn("Email not in whitelist", "email", user.Email)
			// Redirect to frontend with error instead of showing raw HTTP error
			frontendURL := os.Getenv("FRONTEND_URL")
			errorURL := fmt.Sprintf("%s?error=access_denied&error_description=%s",
				frontendURL,
				url.QueryEscape("Your email is not authorized to use this application. Please contact the administrator."))
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

		// Redirect back to frontend (normal web login)
		// Note: FRONTEND_URL is validated at startup in main.go
		frontendURL := os.Getenv("FRONTEND_URL")
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

		// Redirect back to frontend
		// Note: FRONTEND_URL is validated at startup in main.go
		frontendURL := os.Getenv("FRONTEND_URL")
		http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
	}
}

// SessionMiddleware validates web sessions
func SessionMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session cookie
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate session in database
			session, err := database.GetWebSession(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
				return
			}

			// Add user ID to context
			ctx := context.WithValue(r.Context(), userIDContextKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
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

		// Redirect to Google
		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=%s",
			url.QueryEscape(config.GoogleClientID),
			url.QueryEscape(config.GoogleRedirectURL),
			url.QueryEscape(state),
			url.QueryEscape("openid email profile"),
		)
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

		// Check email whitelist
		if !isEmailAllowed(user.Email) {
			logger.Warn("Email not in whitelist", "email", user.Email)
			errorURL := fmt.Sprintf("%s?error=access_denied&error_description=%s",
				frontendURL,
				url.QueryEscape("Your email is not authorized to use this application. Please contact the administrator."))
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

		// Redirect to frontend
		http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
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
		// Preserve any redirect URL in query params
		redirectAfter := r.URL.Query().Get("redirect")

		html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign In - Confab</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: system-ui, -apple-system, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #0f0f0f;
            color: #fff;
        }
        .container {
            background: #1a1a1a;
            padding: 2.5rem;
            border-radius: 1rem;
            box-shadow: 0 20px 60px rgba(0,0,0,0.5);
            text-align: center;
            max-width: 400px;
            width: 90%;
        }
        h1 {
            margin: 0 0 0.5rem 0;
            font-size: 1.5rem;
            color: #fff;
        }
        p {
            color: #888;
            margin: 0 0 2rem 0;
            font-size: 0.9rem;
        }
        .buttons {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }
        .btn {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 0.75rem;
            padding: 0.875rem 1.5rem;
            border: none;
            border-radius: 0.5rem;
            font-size: 1rem;
            font-weight: 500;
            cursor: pointer;
            text-decoration: none;
            transition: transform 0.1s, box-shadow 0.1s;
        }
        .btn:hover {
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.3);
        }
        .btn:active {
            transform: translateY(0);
        }
        .btn-github {
            background: #24292e;
            color: #fff;
            border: 1px solid #444;
        }
        .btn-github:hover {
            background: #2f363d;
        }
        .btn-google {
            background: #fff;
            color: #333;
        }
        .btn-google:hover {
            background: #f5f5f5;
        }
        .icon {
            width: 20px;
            height: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Sign in to Confab</h1>
        <p>Choose your authentication method</p>
        <div class="buttons">
            <a href="/auth/github/login` + func() string {
			if redirectAfter != "" {
				return "?redirect=" + url.QueryEscape(redirectAfter)
			}
			return ""
		}() + `" class="btn btn-github">
                <svg class="icon" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
                </svg>
                Continue with GitHub
            </a>
            <a href="/auth/google/login` + func() string {
			if redirectAfter != "" {
				return "?redirect=" + url.QueryEscape(redirectAfter)
			}
			return ""
		}() + `" class="btn btn-google">
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

		// Get session cookie (user must be logged in via web)
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			// Redirect to login selector, then back here
			redirectURL := "/auth/cli/authorize?" + r.URL.RawQuery
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
			redirectURL := "/auth/cli/authorize?" + r.URL.RawQuery
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

// isEmailAllowed checks if an email is in the whitelist
// If ALLOWED_EMAILS is not set, all emails are allowed (open registration)
// If ALLOWED_EMAILS is set, only those emails can sign up/login
func isEmailAllowed(email string) bool {
	allowedEmailsEnv := os.Getenv("ALLOWED_EMAILS")

	// If no whitelist configured, allow all emails
	if allowedEmailsEnv == "" {
		return true
	}

	// Empty email never allowed
	if email == "" {
		return false
	}

	// Parse comma-separated list
	allowedEmails := strings.Split(allowedEmailsEnv, ",")

	// Check if email is in whitelist (case-insensitive)
	emailLower := strings.ToLower(strings.TrimSpace(email))
	for _, allowed := range allowedEmails {
		allowedLower := strings.ToLower(strings.TrimSpace(allowed))
		if allowedLower == emailLower {
			return true
		}
	}

	return false
}
