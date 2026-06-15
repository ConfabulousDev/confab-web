package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

const (
	SessionCookieName = "confab_session"
	SessionDuration   = 7 * 24 * time.Hour // 7 days, absolute cap
	// DefaultSessionIdleTimeout is the sliding idle window (60j6). A session
	// inactive longer than this is rejected even within the 7-day absolute cap.
	// Overridable via SESSION_IDLE_TIMEOUT (time.ParseDuration format).
	DefaultSessionIdleTimeout = 48 * time.Hour
	// OAuthAPITimeout is the timeout for GitHub OAuth API calls
	// Protects against hanging indefinitely if GitHub API is slow/unresponsive
	OAuthAPITimeout = 30 * time.Second
)

// resolveSessionIdleTimeout reads SESSION_IDLE_TIMEOUT (time.ParseDuration
// format, e.g. "48h", "30m"), falling back to DefaultSessionIdleTimeout on an
// empty, unparseable, or non-positive value (warn + default, mirroring the
// MAX_USERS pattern). A non-positive resolved value never escapes this helper,
// so only the explicit demo sentinel (0) disables the idle gate in GetWebSession.
func resolveSessionIdleTimeout(log *slog.Logger) time.Duration {
	raw := os.Getenv("SESSION_IDLE_TIMEOUT")
	if raw == "" {
		return DefaultSessionIdleTimeout
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		log.Warn("Invalid SESSION_IDLE_TIMEOUT value, using default",
			"value", raw, "default", DefaultSessionIdleTimeout, "error", err)
		return DefaultSessionIdleTimeout
	}
	return parsed
}

// cookieSecure returns whether cookies should have Secure flag
// Secure by default (HTTPS only), can be disabled for local dev
func cookieSecure() bool {
	// Only disable in local development - name is intentionally scary
	return os.Getenv("INSECURE_DEV_MODE") != "true"
}

// clearCookie clears a cookie by setting it with an empty value and MaxAge -1.
// Includes HttpOnly, Secure, and SameSite flags for defense-in-depth.
func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// handleCLIRedirect checks for a cli_redirect cookie and, if valid, redirects to it.
// Returns true if a redirect was performed (caller should return immediately).
// SECURITY: Only allows redirects to /auth/cli/ paths to prevent open redirects.
func handleCLIRedirect(w http.ResponseWriter, r *http.Request, statusCode int) bool {
	cliRedirect, err := r.Cookie("cli_redirect")
	if err != nil || cliRedirect.Value == "" {
		return false
	}
	clearCookie(w, "cli_redirect")
	if strings.HasPrefix(cliRedirect.Value, "/auth/cli/") {
		http.Redirect(w, r, cliRedirect.Value, statusCode)
		return true
	}
	logger.Ctx(r.Context()).Warn("Blocked invalid cli_redirect", "value", cliRedirect.Value)
	return false
}

// checkExpectedEmailMismatch reads the expected_email cookie and checks if the
// user's actual email matches. Returns the expected email and whether there was
// a mismatch. Always clears the cookie.
func checkExpectedEmailMismatch(w http.ResponseWriter, r *http.Request, actualEmail, provider string) (expectedEmail string, mismatch bool) {
	cookie, err := r.Cookie("expected_email")
	if err != nil || cookie.Value == "" {
		return "", false
	}
	expectedEmail = cookie.Value
	clearCookie(w, "expected_email")
	if !strings.EqualFold(expectedEmail, actualEmail) {
		logger.Ctx(r.Context()).Warn("OAuth email mismatch",
			"expected_email", expectedEmail,
			"actual_email", actualEmail,
			"provider", provider)
		return expectedEmail, true
	}
	return expectedEmail, false
}

// appendEmailMismatchParams appends email mismatch query parameters to a URL if needed.
func appendEmailMismatchParams(baseURL, expectedEmail, actualEmail string) string {
	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	return baseURL + separator + "email_mismatch=1&expected=" + url.QueryEscape(expectedEmail) + "&actual=" + url.QueryEscape(actualEmail)
}

// handlePostLoginRedirect performs the standard post-login redirect sequence:
// 1. CLI redirect cookie
// 2. Post-login redirect cookie (e.g., from /device page)
// 3. Default: redirect to frontend
//
// Handles email mismatch parameters throughout. Returns after writing the redirect.
func handlePostLoginRedirect(w http.ResponseWriter, r *http.Request, frontendURL, actualEmail, expectedEmail string, emailMismatch bool) {
	log := logger.Ctx(r.Context())

	// Check if this was a CLI login flow
	if handleCLIRedirect(w, r, http.StatusTemporaryRedirect) {
		return
	}

	// Check if there's a post-login redirect (e.g., from /device page or protected frontend route)
	if postLoginRedirect, err := r.Cookie("post_login_redirect"); err == nil && postLoginRedirect.Value != "" {
		clearCookie(w, "post_login_redirect")
		redirectURL := postLoginRedirect.Value
		// SECURITY: Only allow relative paths to prevent open redirect attacks
		if !strings.HasPrefix(redirectURL, "/") || strings.HasPrefix(redirectURL, "//") {
			log.Warn("Blocked potential open redirect", "redirect_url", redirectURL)
			redirectURL = "/"
		}
		// If it's a frontend path (not a backend path like /device), prepend frontend URL
		if !strings.HasPrefix(redirectURL, "/auth") && !strings.HasPrefix(redirectURL, "/device") {
			redirectURL = frontendURL + redirectURL
		}
		if emailMismatch {
			redirectURL = appendEmailMismatchParams(redirectURL, expectedEmail, actualEmail)
		}
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// Default: redirect to frontend
	finalURL := frontendURL
	if emailMismatch {
		finalURL = appendEmailMismatchParams(finalURL, expectedEmail, actualEmail)
	}
	http.Redirect(w, r, finalURL, http.StatusTemporaryRedirect)
}

// validateOAuthCallback performs the state+PKCE+code validation shared by every
// OAuth callback (GitHub/Google/OIDC). It checks the CSRF state cookie against
// the state query param, requires a non-empty single-use PKCE verifier cookie
// (r9zn), clears both cookies, and returns the authorization code + verifier.
//
// On any validation failure it writes the appropriate 400 response itself
// (matching the historical inline behavior: "Invalid state parameter" for bad
// state or missing verifier, "Missing code parameter" for an absent code) and
// returns a non-nil error so the caller returns immediately without inspecting
// the error.
func validateOAuthCallback(w http.ResponseWriter, r *http.Request) (code, verifier string, err error) {
	// Validate state to prevent CSRF
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return "", "", fmt.Errorf("invalid oauth state")
	}

	// Clear state cookie
	clearCookie(w, "oauth_state")

	// PKCE: the verifier cookie must be present; it is single-use and binds
	// the auth code to this browser (r9zn). Same 400 shape as invalid state.
	verifierCookie, err := r.Cookie("oauth_verifier")
	if err != nil || verifierCookie.Value == "" {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return "", "", fmt.Errorf("missing oauth verifier")
	}
	clearCookie(w, "oauth_verifier")

	// Get authorization code
	code = r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return "", "", fmt.Errorf("missing oauth code")
	}

	return code, verifierCookie.Value, nil
}

// Sentinel errors returned by checkUserEligibility so callbacks can map each
// failure mode to its historical redirect + log shape.
var (
	// errEmailDomainNotPermitted: the email's domain is outside the configured
	// allow-list (or the email itself is invalid). Maps to access_denied.
	errEmailDomainNotPermitted = errors.New("email domain not permitted")
	// errUserCapReached: the user cap (MAX_USERS) blocks this new login.
	// Maps to access_denied.
	errUserCapReached = errors.New("user cap reached")
)

// checkUserEligibility performs the email-domain allow-list + user-cap checks
// shared by every OAuth callback (GitHub/Google/OIDC). It returns:
//   - errEmailDomainNotPermitted if the email's domain is not allowed,
//   - errUserCapReached if the user cap blocks a new login,
//   - the underlying error from CanUserLogin if the eligibility check itself
//     fails (caller maps this to a server_error redirect),
//   - nil if the user may proceed.
//
// The domain check runs first and short-circuits before any DB access.
func checkUserEligibility(ctx context.Context, database *db.DB, email string, allowedDomains []string) error {
	if !validation.IsAllowedEmailDomain(email, allowedDomains) {
		return errEmailDomainNotPermitted
	}

	allowed, err := CanUserLogin(ctx, database, email)
	if err != nil {
		return err
	}
	if !allowed {
		return errUserCapReached
	}
	return nil
}

// redirectUserIneligible maps a checkUserEligibility error to the historical
// per-provider log line + login-page redirect, then writes the redirect. It is
// the caller's single response path when eligibility fails.
func redirectUserIneligible(w http.ResponseWriter, r *http.Request, frontendURL, provider, email string, err error) {
	log := logger.Ctx(r.Context())

	// Each branch picks the login-page error code + description for its failure
	// mode; the redirect shape (and 307 status) is identical across all three.
	var errorCode, description string
	switch {
	case errors.Is(err, errEmailDomainNotPermitted):
		log.Warn("Email domain not permitted", "email", email, "provider", provider)
		errorCode = "access_denied"
		description = "Your email domain is not permitted. Contact your administrator."
	case errors.Is(err, errUserCapReached):
		log.Warn("User cap reached, login denied", "email", email)
		errorCode = "access_denied"
		description = "This application has reached its user limit. Please contact the administrator."
	default:
		log.Error("Failed to check user login eligibility", "error", err, "email", email)
		errorCode = "server_error"
		description = "An error occurred. Please try again later."
	}

	errorURL := fmt.Sprintf("%s/login?error=%s&error_description=%s",
		frontendURL, errorCode, url.QueryEscape(description))
	http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
}

// redirectInactiveUser sends a deactivated user back to /login with a generic
// "contact support" message (w8tz). The three OAuth callbacks call this after
// FindOrCreateUserByOAuth resolves to a user whose status is inactive, BEFORE
// CreateWebSession — so no session is minted and the app→401→login→app loop can
// never start (re-login no longer silently succeeds for a deactivated account).
//
// The copy is deliberately generic: it does not confirm the account is
// deactivated (the ticket asks not to reveal too much), matching the
// account-state opacity the password path already has via ErrInvalidCredentials.
// Centralized so the three callbacks stay identical and a copy change touches
// one place. Caller must already have logged the rejection.
func redirectInactiveUser(w http.ResponseWriter, r *http.Request, frontendURL string) {
	const message = "Your account is not active. Please contact support."
	errorURL := fmt.Sprintf("%s/login?error=account_inactive&error_description=%s",
		frontendURL, url.QueryEscape(message))
	http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
}

// oauthHTTPClient returns an HTTP client with timeout for OAuth API calls
func oauthHTTPClient() *http.Client {
	return &http.Client{
		Timeout: OAuthAPITimeout,
	}
}

// generatePKCE returns an RFC 7636 S256 PKCE pair: a high-entropy code verifier
// (32 random bytes, base64url, no padding) and its challenge
// (base64url(SHA256(verifier)), no padding). RawURLEncoding keeps both inside the
// RFC 7636 unreserved charset so they need no escaping in URLs or form bodies.
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// setOAuthLoginCookies sets the standard pre-login cookies (CSRF state, PKCE
// verifier, post-login redirect, expected email) that all OAuth login handlers
// need. Returns the state token, the PKCE code_challenge to append to the
// auth-init URL, and whether a valid email hint was provided. The verifier
// itself is stored in the HttpOnly `oauth_verifier` cookie (single-use, cleared
// on callback) so a stolen authorization code can't be exchanged without it (r9zn, A1).
func setOAuthLoginCookies(w http.ResponseWriter, r *http.Request) (state, challenge string, validEmail bool, expectedEmail string, err error) {
	state, err = generateRandomString(32)
	if err != nil {
		return "", "", false, "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", "", false, "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_verifier",
		Value:    verifier,
		Path:     "/",
		MaxAge:   300, // 5 minutes, mirrors oauth_state
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})

	if redirectAfter := r.URL.Query().Get("redirect"); redirectAfter != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "post_login_redirect",
			Value:    redirectAfter,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			Secure:   cookieSecure(),
			SameSite: http.SameSiteLaxMode,
		})
	}

	expectedEmail = r.URL.Query().Get("email")
	validEmail = expectedEmail != "" && validation.IsValidEmail(expectedEmail)
	if validEmail {
		http.SetCookie(w, &http.Cookie{
			Name:     "expected_email",
			Value:    expectedEmail,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			Secure:   cookieSecure(),
			SameSite: http.SameSiteLaxMode,
		})
	}

	return state, challenge, validEmail, expectedEmail, nil
}

// OAuthConfig holds OAuth configuration for all providers
type OAuthConfig struct {
	// Password authentication
	PasswordEnabled bool

	// GitHub OAuth (optional)
	GitHubEnabled      bool
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string

	// Google OAuth (optional)
	GoogleEnabled      bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Generic OIDC (optional) — works with Okta, Auth0, Azure AD, Keycloak, etc.
	OIDCEnabled      bool
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCIssuerURL    string // raw issuer URL for lazy discovery
	OIDCDisplayName  string // button text, default "SSO"

	// Email domain restrictions (optional, for on-prem deployments)
	AllowedEmailDomains []string

	// AutoLinkEmail (OAUTH_AUTO_LINK_EMAIL, default false) controls whether a
	// first-time OAuth login whose email matches an existing account is
	// automatically linked to it. Default OFF prevents account takeover when an
	// attacker controls a matching IdP email; when off, the collision errors to
	// the login page instead of linking (cm4f).
	AutoLinkEmail bool

	// CF-483: Demo mode. When DemoIdentityEmail is set, anonymous web
	// visitors on auth-required routes are auto-impersonated as the
	// designated demo user (which is per-user read-only). CSRFSecretKey
	// is reused as the HMAC key for the shared demo session cookie ID.
	// Both fields empty = zero behavior change.
	DemoIdentityEmail string
	CSRFSecretKey     string

	oidcEndpoints *OIDCEndpoints // lazily populated, cached on success only
	oidcMu        sync.Mutex     // protects lazy discovery
}

// HandleLogout logs out the user.
//
// CF-483 B2: when the cookie value is the shared demo session ID, we
// clear the client cookie but skip the DB delete — otherwise the next
// anonymous visitor would briefly fail auto-impersonate until the row
// is re-upserted, and we'd thrash the row on every demo logout.
// DemoSessionCookieID returns "" when DemoIdentityEmail is unset, so
// the comparison is inert in non-demo deployments.
func HandleLogout(database *db.DB, config *OAuthConfig) http.HandlerFunc {
	authStore := &dbauth.Store{DB: database}
	demoSessionID := DemoSessionCookieID(config.CSRFSecretKey, config.DemoIdentityEmail)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logger.Ctx(ctx)

		cookie, err := r.Cookie(SessionCookieName)

		// Clear cookie first — this always succeeds (Set-Cookie header) and ensures
		// the user is logged out even if the DB cleanup fails.
		clearCookie(w, SessionCookieName)

		if err == nil {
			switch {
			case demoSessionID != "" && cookie.Value == demoSessionID:
				log.Info("demo logout: clearing cookie but preserving shared session row")
			default:
				if err := authStore.DeleteWebSession(ctx, cookie.Value); err != nil {
					log.Warn("Failed to delete web session from database", "error", err)
				}
			}
		}

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
			log.Warn("Blocked potential open redirect in logout", "redirect_url", redirectAfter)
		}

		// Redirect back to frontend
		// Note: FRONTEND_URL is validated at startup in main.go
		http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
	}
}

// sessionAuthResult contains the result of session authentication
type sessionAuthResult struct {
	userID       int64
	userEmail    string
	userReadOnly bool // CF-483: stashed in request ctx for EnforceReadOnly
}

// TrySessionAuth attempts to authenticate using a session cookie.
// Returns the auth result if successful, nil otherwise.
// Does not reject - callers decide whether to require auth.
func TrySessionAuth(r *http.Request, database *db.DB) *sessionAuthResult {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil
	}

	authStore := &dbauth.Store{DB: database}
	session, err := authStore.GetWebSession(r.Context(), cookie.Value, resolveSessionIdleTimeout(logger.Ctx(r.Context())))
	if err != nil {
		return nil
	}

	// Check if user is inactive. This is the security-relevant case: a session
	// cookie that resolves to a deactivated account. Log it decisively (the
	// other nil returns above are ordinary anonymous/expired traffic and stay
	// silent to avoid spamming on every unauthenticated request).
	if session.UserStatus == models.UserStatusInactive {
		logger.Ctx(r.Context()).Warn("Session auth rejected: user inactive",
			"reason", "user_inactive",
			"user_id", session.UserID,
			"client_ip", clientip.FromRequest(r).Primary,
			"method", r.Method,
			"path", r.URL.Path)
		return nil
	}

	return &sessionAuthResult{userID: session.UserID, userEmail: session.UserEmail, userReadOnly: session.ReadOnly}
}

// RequireSession returns an HTTP middleware that requires session cookie authentication.
// If allowedDomains is non-empty, the user's email domain must match.
// Use TrySessionAuth for optional authentication.
//
// CF-483: when config.DemoIdentityEmail is set and no real session is
// present, falls back to AutoImpersonateIfDemo so anonymous browser
// visitors are seen as the read-only demo user. Chains EnforceReadOnly
// internally so mutating requests from the demo identity get the
// documented 403. Both are inert when DemoIdentityEmail is empty.
func RequireSession(database *db.DB, config *OAuthConfig) func(http.Handler) http.Handler {
	enforceReadOnly := EnforceReadOnly(database)
	return func(next http.Handler) http.Handler {
		next = enforceReadOnly(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authResult := TrySessionAuth(r, database)
			if authResult == nil {
				authResult = AutoImpersonateIfDemo(w, r, database, config.DemoIdentityEmail, config.CSRFSecretKey)
			}
			if authResult == nil {
				logger.Ctx(r.Context()).Warn("Session auth rejected",
					"reason", "no_valid_session",
					"client_ip", clientip.FromRequest(r).Primary,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check email domain restriction
			if !validation.IsAllowedEmailDomain(authResult.userEmail, config.AllowedEmailDomains) {
				logger.Ctx(r.Context()).Warn("Session auth rejected: email domain not permitted",
					"reason", "email_domain_not_permitted",
					"user_id", authResult.userID,
					"client_ip", clientip.FromRequest(r).Primary,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Email domain not permitted", http.StatusForbidden)
				return
			}

			// Set user ID on logger's response writer
			setLogUserID(w, authResult.userID)

			// Enrich request-scoped logger with user_id
			log := logger.Ctx(r.Context()).With("user_id", authResult.userID)
			ctx := logger.WithLogger(r.Context(), log)

			// Enrich OpenTelemetry span with user info
			enrichSpanWithUser(ctx, authResult.userID, authResult.userEmail, false, true)

			// Add user ID + read-only flag (CF-483) to context
			ctx = context.WithValue(ctx, userIDContextKey, authResult.userID)
			ctx = WithReadOnly(ctx, authResult.userReadOnly)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSessionOrAPIKey returns an HTTP middleware that requires either
// session cookie or API key authentication. Tries session first, then API key.
// If allowedDomains is non-empty, the user's email domain must match.
//
// CF-483: when config.DemoIdentityEmail is set and neither real auth path
// succeeds, falls back to auto-impersonate as the read-only demo user.
// Chains EnforceReadOnly internally.
func RequireSessionOrAPIKey(database *db.DB, config *OAuthConfig) func(http.Handler) http.Handler {
	enforceReadOnly := EnforceReadOnly(database)
	return func(next http.Handler) http.Handler {
		next = enforceReadOnly(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var userID int64
			var userEmail string
			var userReadOnly bool
			var authAPIKey, authSession bool

			// Try session cookie first
			if sessionAuth := TrySessionAuth(r, database); sessionAuth != nil {
				userID = sessionAuth.userID
				userEmail = sessionAuth.userEmail
				userReadOnly = sessionAuth.userReadOnly
				authSession = true
			} else if apiKeyAuth := TryAPIKeyAuth(r, database); apiKeyAuth != nil {
				// Fall back to API key
				userID = apiKeyAuth.userID
				userEmail = apiKeyAuth.userEmail
				userReadOnly = apiKeyAuth.userReadOnly
				authAPIKey = true
			} else if demoAuth := AutoImpersonateIfDemo(w, r, database, config.DemoIdentityEmail, config.CSRFSecretKey); demoAuth != nil {
				userID = demoAuth.userID
				userEmail = demoAuth.userEmail
				userReadOnly = demoAuth.userReadOnly
				authSession = true
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check email domain restriction
			if !validation.IsAllowedEmailDomain(userEmail, config.AllowedEmailDomains) {
				http.Error(w, "Email domain not permitted", http.StatusForbidden)
				return
			}

			// Set user ID on logger's response writer
			setLogUserID(w, userID)

			// Enrich request-scoped logger with user_id
			log := logger.Ctx(r.Context()).With("user_id", userID)
			ctx := logger.WithLogger(r.Context(), log)

			// Enrich OpenTelemetry span with user info
			enrichSpanWithUser(ctx, userID, userEmail, authAPIKey, authSession)

			// Add user ID + read-only flag (CF-483) to context
			ctx = context.WithValue(ctx, userIDContextKey, userID)
			ctx = WithReadOnly(ctx, userReadOnly)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth returns an HTTP middleware that attempts authentication but doesn't require it.
// If authentication succeeds (via session cookie or API key), the user ID is set in context.
// If authentication fails, the request continues without a user ID.
// If allowedDomains is non-empty and a user is authenticated, their email domain must match or they get 403.
// Use auth.GetUserID(ctx) to check if a user is authenticated.
//
// CF-483: when config.DemoIdentityEmail is set, anonymous requests are
// auto-impersonated as the read-only demo user (rather than continuing
// without a user ID). Chains EnforceReadOnly internally. This is what
// makes the read-only demo flow work for "canonical access" endpoints
// like GET /sessions/{id}.
func OptionalAuth(database *db.DB, config *OAuthConfig) func(http.Handler) http.Handler {
	enforceReadOnly := EnforceReadOnly(database)
	return func(next http.Handler) http.Handler {
		next = enforceReadOnly(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var userID int64
			var userEmail string
			var userReadOnly bool
			var authAPIKey, authSession bool

			// Try API key first, then session cookie
			if apiKeyAuth := TryAPIKeyAuth(r, database); apiKeyAuth != nil {
				userID = apiKeyAuth.userID
				userEmail = apiKeyAuth.userEmail
				userReadOnly = apiKeyAuth.userReadOnly
				authAPIKey = true
			} else if sessionAuth := TrySessionAuth(r, database); sessionAuth != nil {
				userID = sessionAuth.userID
				userEmail = sessionAuth.userEmail
				userReadOnly = sessionAuth.userReadOnly
				authSession = true
			} else if demoAuth := AutoImpersonateIfDemo(w, r, database, config.DemoIdentityEmail, config.CSRFSecretKey); demoAuth != nil {
				userID = demoAuth.userID
				userEmail = demoAuth.userEmail
				userReadOnly = demoAuth.userReadOnly
				authSession = true
			} else {
				// No auth - when domain restrictions are in place, require authentication
				// to prevent anonymous access to public shares on on-prem instances
				if len(config.AllowedEmailDomains) > 0 {
					http.Error(w, "Authentication required", http.StatusUnauthorized)
					return
				}
				// No auth and no domain restrictions - continue without user ID in context
				next.ServeHTTP(w, r)
				return
			}

			if !validation.IsAllowedEmailDomain(userEmail, config.AllowedEmailDomains) {
				http.Error(w, "Email domain not permitted", http.StatusForbidden)
				return
			}

			setLogUserID(w, userID)
			log := logger.Ctx(r.Context()).With("user_id", userID)
			ctx := logger.WithLogger(r.Context(), log)
			enrichSpanWithUser(ctx, userID, userEmail, authAPIKey, authSession)
			ctx = context.WithValue(ctx, userIDContextKey, userID)
			ctx = WithReadOnly(ctx, userReadOnly)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getOIDCEndpoints lazily discovers OIDC endpoints on first call.
// Thread-safe via mutex. Only caches on success — retries on failure
// so a temporary IdP outage doesn't permanently break OIDC.
func (c *OAuthConfig) getOIDCEndpoints() (*OIDCEndpoints, error) {
	c.oidcMu.Lock()
	defer c.oidcMu.Unlock()

	if c.oidcEndpoints != nil {
		return c.oidcEndpoints, nil
	}

	endpoints, err := DiscoverOIDC(c.OIDCIssuerURL)
	if err != nil {
		return nil, err
	}

	c.oidcEndpoints = endpoints
	return endpoints, nil
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
	authStore := &dbauth.Store{DB: database}
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())
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
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		// Validate session
		session, err := authStore.GetWebSession(ctx, cookie.Value, resolveSessionIdleTimeout(log))
		if err != nil {
			// Session is invalid or expired - clear the stale cookie and redirect to login
			clearCookie(w, SessionCookieName)

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
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		// CF-483 B1: never mint API keys for the demo (read-only) user.
		// Even though the auth/cli/* endpoints are not auto-impersonated,
		// a visitor who first browsed the SPA holds the shared demo
		// cookie and would otherwise mint API keys here.
		if session.ReadOnly {
			log.Warn("CLI authorize blocked for read-only user", "user_id", session.UserID)
			clearCookie(w, SessionCookieName)
			frontendURL := os.Getenv("FRONTEND_URL")
			errorURL := fmt.Sprintf("%s/login?error=access_denied&error_description=%s",
				frontendURL,
				url.QueryEscape("This identity cannot create API keys. Log in with your own account."))
			http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
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
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
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

		// Replace existing API key with same name, or create new one
		// This prevents unbounded key growth when re-authenticating from the same machine
		keyID, createdAt, err := authStore.ReplaceAPIKey(ctx, session.UserID, keyHash, keyName)
		if err != nil {
			if err == db.ErrAPIKeyLimitExceeded {
				// Redirect to callback with error that CLI can handle
				frontendURL := os.Getenv("FRONTEND_URL")
				redirectURL := fmt.Sprintf("%s?error=api_key_limit_exceeded", callback)
				log.Warn("API key limit exceeded", "user_id", session.UserID)
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
        <p>You have reached the maximum number of API keys. Please delete some unused keys before creating new ones.</p>
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
			log.Error("Failed to create API key in database", "error", err, "user_id", session.UserID)
			http.Error(w, "Failed to create API key", http.StatusInternalServerError)
			return
		}

		log.Info("API key created successfully",
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
	log := logger.Ctx(ctx)

	if database == nil {
		return false, fmt.Errorf("database is required")
	}

	// Validate email format (also rejects empty and whitespace-only emails)
	if !validation.IsValidEmail(email) {
		return false, nil
	}

	userStore := &dbuser.Store{DB: database}

	// Check if user already exists - returning users always allowed
	exists, err := userStore.UserExistsByEmail(ctx, email)
	if err != nil {
		log.Warn("Failed to check if user exists", "email", email, "error", err)
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
			log.Warn("Invalid MAX_USERS value, using default", "value", maxUsersEnv, "default", DefaultMaxUsers, "error", err)
		} else {
			maxUsers = parsed
		}
	}

	currentUsers, err := userStore.CountUsers(ctx)
	if err != nil {
		log.Warn("Failed to count users", "error", err)
		return false, err
	}

	if currentUsers >= maxUsers {
		log.Warn("User cap reached", "current", currentUsers, "max", maxUsers, "email", email)
		return false, nil
	}

	return true, nil
}
