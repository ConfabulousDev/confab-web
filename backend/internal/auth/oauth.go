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
	"time"

	"github.com/santaclaude2025/confab/backend/internal/db"
)

const (
	SessionCookieName = "confab_session"
	SessionDuration   = 7 * 24 * time.Hour // 7 days
)

// cookieSecure returns whether cookies should have Secure flag
// Secure by default (HTTPS only), can be disabled for local dev
func cookieSecure() bool {
	// Only disable in local development - name is intentionally scary
	return os.Getenv("INSECURE_DEV_MODE") != "true"
}

// OAuthConfig holds OAuth configuration
type OAuthConfig struct {
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
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
			http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
			return
		}

		// Get user info from GitHub
		user, err := getGitHubUser(accessToken)
		if err != nil {
			fmt.Printf("Error getting GitHub user: %v\n", err)
			http.Error(w, "Failed to get user info", http.StatusInternalServerError)
			return
		}

		// Debug: log user info from GitHub
		fmt.Printf("GitHub user: ID=%d, Login=%s, Email=%s, Name=%s\n", user.ID, user.Login, user.Email, user.Name)

		// Use login (username) as fallback if name is empty
		displayName := user.Name
		if displayName == "" {
			displayName = user.Login
		}

		// Find or create user in database
		githubID := fmt.Sprintf("%d", user.ID)
		dbUser, err := database.FindOrCreateUserByGitHub(ctx, githubID, user.Login, user.Email, displayName, user.AvatarURL)
		if err != nil {
			fmt.Printf("Error creating user: %v\n", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Create web session
		sessionID, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().Add(SessionDuration)
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

		// Redirect back to frontend
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5173"
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

		// Redirect back to frontend
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5173"
		}
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

	resp, err := http.DefaultClient.Do(req)
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// Get email if not provided
	if user.Email == "" {
		email, _ := getGitHubPrimaryEmail(accessToken)
		user.Email = email
	}

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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []GitHubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", fmt.Errorf("no email found")
}

// generateRandomString generates a random string for sessions/state
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// HandleCLIAuthorize handles CLI API key generation flow
func HandleCLIAuthorize(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get session cookie (user must be logged in via web)
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			// Redirect to GitHub login, then back here
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
			http.Redirect(w, r, "/auth/github/login", http.StatusTemporaryRedirect)
			return
		}

		// Validate session
		session, err := database.GetWebSession(ctx, cookie.Value)
		if err != nil {
			http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
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
			fmt.Printf("Error creating API key: %v\n", err)
			http.Error(w, "Failed to create API key", http.StatusInternalServerError)
			return
		}

		fmt.Printf("Created API key: ID=%d, Name=%s, UserID=%d, CreatedAt=%v\n", keyID, keyName, session.UserID, createdAt)

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
