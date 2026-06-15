package auth

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

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
	ExpiresIn       int    `json:"expires_in"` // seconds
	Interval        int    `json:"interval"`   // polling interval in seconds
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
	// Rejection sampling: discard random bytes in the biased tail so every
	// symbol is equiprobable. A plain `b % 31` over 0..255 favors the first
	// 256 % 31 = 8 symbols (8epk); only bytes in [0, limit) map uniformly.
	limit := byte(256 - (256 % len(chars))) // 248
	code := make([]byte, 8)
	for i := range code {
		var b [1]byte
		for {
			if _, err := rand.Read(b[:]); err != nil {
				return "", err
			}
			if b[0] < limit {
				break
			}
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
	authStore := &dbauth.Store{DB: database}
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())
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
			log.Error("Failed to generate device code", "error", err)
			http.Error(w, "Failed to generate device code", http.StatusInternalServerError)
			return
		}

		userCode, err := generateUserCode()
		if err != nil {
			log.Error("Failed to generate user code", "error", err)
			http.Error(w, "Failed to generate user code", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(DeviceCodeExpiry)

		// Store in database
		if err := authStore.CreateDeviceCode(ctx, deviceCode, userCode, req.KeyName, expiresAt); err != nil {
			log.Error("Failed to store device code", "error", err)
			http.Error(w, "Failed to create device code", http.StatusInternalServerError)
			return
		}

		log.Info("Device code created", "user_code", userCode)

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

// writeDeviceTokenError writes a JSON error response for the device token endpoint.
func writeDeviceTokenError(w http.ResponseWriter, statusCode int, errorCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(DeviceTokenResponse{Error: errorCode})
}

// HandleDeviceToken exchanges a device code for an API key
// POST /auth/device/token
func HandleDeviceToken(database *db.DB, allowedDomains []string) http.HandlerFunc {
	authStore := &dbauth.Store{DB: database}
	userStore := &dbuser.Store{DB: database}
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())
		ctx := r.Context()

		// Parse request
		var req DeviceTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDeviceTokenError(w, http.StatusBadRequest, "invalid_request")
			return
		}

		if req.DeviceCode == "" {
			writeDeviceTokenError(w, http.StatusBadRequest, "invalid_request")
			return
		}

		// Look up device code
		dc, err := authStore.GetDeviceCodeByDeviceCode(ctx, req.DeviceCode)
		if err != nil {
			if err == db.ErrDeviceCodeNotFound {
				writeDeviceTokenError(w, http.StatusBadRequest, "invalid_grant")
			} else {
				writeDeviceTokenError(w, http.StatusInternalServerError, "server_error")
			}
			return
		}

		// Check if expired
		if time.Now().UTC().After(dc.ExpiresAt) {
			authStore.DeleteDeviceCode(ctx, req.DeviceCode)
			writeDeviceTokenError(w, http.StatusBadRequest, "expired_token")
			return
		}

		// Check if authorized
		if dc.AuthorizedAt == nil || dc.UserID == nil {
			writeDeviceTokenError(w, http.StatusBadRequest, "authorization_pending")
			return
		}

		// Check email domain restriction on the authorized user
		if len(allowedDomains) > 0 {
			user, err := userStore.GetUserByID(ctx, *dc.UserID)
			if err != nil {
				log.Error("Failed to get user for domain check", "error", err, "user_id", *dc.UserID)
				writeDeviceTokenError(w, http.StatusInternalServerError, "server_error")
				return
			}
			if !validation.IsAllowedEmailDomain(user.Email, allowedDomains) {
				log.Warn("Email domain not permitted in device flow", "email", user.Email, "user_id", *dc.UserID)
				authStore.DeleteDeviceCode(ctx, req.DeviceCode)
				writeDeviceTokenError(w, http.StatusForbidden, "access_denied")
				return
			}
		}

		// Authorized! Generate API key
		apiKey, keyHash, err := GenerateAPIKey()
		if err != nil {
			log.Error("Failed to generate API key", "error", err)
			writeDeviceTokenError(w, http.StatusInternalServerError, "server_error")
			return
		}

		// Replace existing API key with same name, or create new one
		// This prevents unbounded key growth when re-authenticating from the same machine
		keyID, createdAt, err := authStore.ReplaceAPIKey(ctx, *dc.UserID, keyHash, dc.KeyName)
		if err != nil {
			if err == db.ErrAPIKeyLimitExceeded {
				log.Warn("API key limit exceeded during device flow", "user_id", *dc.UserID)
				writeDeviceTokenError(w, http.StatusConflict, "api_key_limit_exceeded")
				return
			}
			log.Error("Failed to create API key", "error", err, "user_id", *dc.UserID)
			writeDeviceTokenError(w, http.StatusInternalServerError, "server_error")
			return
		}

		log.Info("API key created via device flow",
			"key_id", keyID,
			"name", dc.KeyName,
			"user_id", *dc.UserID,
			"created_at", createdAt)

		// Delete the device code (one-time use)
		authStore.DeleteDeviceCode(ctx, req.DeviceCode)

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
	authStore := &dbauth.Store{DB: database}
	return func(w http.ResponseWriter, r *http.Request) {
		// Get pre-filled code from query param
		prefilledCode := r.URL.Query().Get("code")

		// Check if user is logged in
		cookie, err := r.Cookie(SessionCookieName)
		loggedIn := err == nil && cookie.Value != ""

		if loggedIn {
			_, err := authStore.GetWebSession(r.Context(), cookie.Value, resolveSessionIdleTimeout(logger.Ctx(r.Context())))
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
			loginURL := "/login?redirect=" + url.QueryEscape(redirectURL)
			http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
			return
		}

		// Logged in - show the code entry form (escape to prevent XSS)
		pageHTML := generateDevicePageHTML(html.EscapeString(prefilledCode))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(pageHTML))
	}
}

// HandleDeviceVerify handles the form submission to verify a device code
// POST /device/verify
func HandleDeviceVerify(database *db.DB, allowedDomains []string) http.HandlerFunc {
	authStore := &dbauth.Store{DB: database}
	// Per-verifier failed-attempt lockout (in-memory, no migration). Keyed by
	// the verifier's user ID so a logged-in attacker can't brute-force
	// outstanding user_codes within the short expiry window (8epk). One
	// instance per handler — created here, shared across requests.
	verifyLimiter := newAttemptLimiter(deviceVerifyMaxFailures, deviceVerifyLockout)
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())
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
		loginRedirect := "/login?redirect=" + url.QueryEscape(redirectURL)

		// Must be logged in
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Redirect(w, r, loginRedirect, http.StatusTemporaryRedirect)
			return
		}

		session, err := authStore.GetWebSession(ctx, cookie.Value, resolveSessionIdleTimeout(logger.Ctx(ctx)))
		if err != nil {
			http.Redirect(w, r, loginRedirect, http.StatusTemporaryRedirect)
			return
		}

		// CF-483 B1: never authorize device codes for the demo (read-only)
		// user. Same rationale as HandleCLIAuthorize — a visitor holding
		// the shared demo cookie could otherwise mint a CLI device.
		if session.ReadOnly {
			log.Warn("Device verify blocked for read-only user", "user_id", session.UserID)
			html := generateDeviceResultHTML(false, "This identity cannot authorize devices. Log in with your own account.")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(html))
			return
		}

		// Check email domain restriction before authorizing device code
		if !validation.IsAllowedEmailDomain(session.UserEmail, allowedDomains) {
			log.Warn("Email domain not permitted in device verify", "email", session.UserEmail)
			html := generateDeviceResultHTML(false, "Your email domain is not permitted. Contact your administrator.")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(html))
			return
		}

		// Per-verifier brute-force lockout: too many failed verify attempts by
		// this session locks it out of device-verify for the window (8epk).
		limiterKey := strconv.FormatInt(session.UserID, 10)
		if verifyLimiter.Locked(limiterKey) {
			log.Warn("Device verify locked out after repeated failures", "user_id", session.UserID)
			html := generateDeviceResultHTML(false, "Too many attempts. Please wait a few minutes and try again.")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(html))
			return
		}

		// Normalize: remove any dash-like characters and re-add in correct position
		// Handle various dash characters that might be pasted (hyphen, en-dash, em-dash, etc.)
		for _, dash := range []string{"-", "–", "—", "‐", "−", "‑"} {
			userCode = strings.ReplaceAll(userCode, dash, "")
		}
		if len(userCode) == 8 {
			userCode = userCode[:4] + "-" + userCode[4:]
		}

		// Validate and authorize
		err = authStore.AuthorizeDeviceCode(ctx, userCode, session.UserID)
		if err != nil {
			verifyLimiter.RecordFailure(limiterKey)
			log.Warn("Device code authorization failed", "error", err, "user_code", userCode)
			// Show error page
			html := generateDeviceResultHTML(false, "Invalid or expired code. Please try again.")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(html))
			return
		}
		verifyLimiter.Reset(limiterKey) // successful authorize clears the count

		log.Info("Device code authorized", "user_code", userCode, "user_id", session.UserID)

		// Show success page
		html := generateDeviceResultHTML(true, "Device authorized! You can close this window and return to your terminal.")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// generateDevicePageHTML generates the HTML for the device authorization page.
// NOTE: Inline HTML is intentional here - these are simple, self-contained pages that
// rarely change. Keeping them inline avoids external template file dependencies and
// simplifies deployment. This is acceptable for low-churn auth UI pages.
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
        <p class="hint">The code expires in 5 minutes.</p>
    </div>
</body>
</html>`, prefilledCode)
}

// generateDeviceResultHTML generates the HTML for the device authorization result page.
// NOTE: Inline HTML is intentional - see comment on generateDevicePageHTML.
func generateDeviceResultHTML(success bool, message string) string {
	var icon, iconColor, title, homeLink string
	if success {
		icon = "✓"
		iconColor = "#16a34a"
		title = "Success!"
		if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
			homeLink = fmt.Sprintf(`<a href="%s" class="home-link">Go to Confab</a>`, frontendURL)
		}
	} else {
		icon = "✗"
		iconColor = "#dc2626"
		title = "Error"
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
</html>`, iconColor, icon, title, message, homeLink)
}
