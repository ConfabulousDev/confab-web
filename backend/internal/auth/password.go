package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

const (
	// ProviderPassword is the provider name for password-based auth
	ProviderPassword = "password"

	// BcryptCost is the cost factor for bcrypt hashing
	// 12 is a good balance of security and performance (~250ms on modern hardware)
	BcryptCost = 12
)

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compares a password against a bcrypt hash
// Uses constant-time comparison to prevent timing attacks
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HandlePasswordLogin handles POST /auth/password/login
func HandlePasswordLogin(database *db.DB, allowedDomains []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logger.Ctx(ctx)

		// Parse form
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		email := validation.NormalizeEmail(r.FormValue("email"))
		password := r.FormValue("password")

		// Validate inputs
		if !validation.IsValidEmail(email) {
			redirectWithError(w, r, "Invalid email address")
			return
		}
		if password == "" {
			redirectWithError(w, r, "Password is required")
			return
		}

		// Check email domain restriction
		if !validation.IsAllowedEmailDomain(email, allowedDomains) {
			log.Warn("Email domain not permitted", "email", email, "provider", "password")
			redirectWithError(w, r, "Your email domain is not permitted. Contact your administrator.")
			return
		}

		// Attempt login
		user, err := database.AuthenticatePassword(ctx, email, password)
		if err != nil {
			if errors.Is(err, db.ErrAccountLocked) {
				log.Warn("Login attempt on locked account", "email", email)
				redirectWithError(w, r, "Account is temporarily locked. Please try again later.")
				return
			}
			if errors.Is(err, db.ErrInvalidCredentials) {
				log.Warn("Failed login attempt", "email", email)
				redirectWithError(w, r, "Invalid email or password")
				return
			}
			log.Error("Password authentication error", "error", err, "email", email)
			redirectWithError(w, r, "An error occurred. Please try again.")
			return
		}

		log.Info("Password login successful", "user_id", user.ID, "email", email)

		// Create web session (same as OAuth)
		sessionID, err := generateRandomString(32)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(SessionDuration)
		if err := database.CreateWebSession(ctx, sessionID, user.ID, expiresAt); err != nil {
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

		// Handle post-login redirect (same as OAuth)
		frontendURL := os.Getenv("FRONTEND_URL")

		// Check for post-login redirect cookie
		if postLoginRedirect, err := r.Cookie("post_login_redirect"); err == nil && postLoginRedirect.Value != "" {
			clearCookie(w, "post_login_redirect")
			redirectURL := postLoginRedirect.Value
			// SECURITY: Only allow relative paths
			if strings.HasPrefix(redirectURL, "/") && !strings.HasPrefix(redirectURL, "//") {
				if !strings.HasPrefix(redirectURL, "/auth") && !strings.HasPrefix(redirectURL, "/device") {
					redirectURL = frontendURL + redirectURL
				}
				http.Redirect(w, r, redirectURL, http.StatusSeeOther)
				return
			}
		}

		// CLI login flow
		if handleCLIRedirect(w, r, http.StatusSeeOther) {
			return
		}

		// Default: redirect to frontend
		http.Redirect(w, r, frontendURL, http.StatusSeeOther)
	}
}

// redirectWithError redirects back to login page with an error message
func redirectWithError(w http.ResponseWriter, r *http.Request, message string) {
	loginURL := "/login?error=" + url.QueryEscape(message)

	// Preserve redirect parameter if present
	if redirect := r.FormValue("redirect"); redirect != "" {
		loginURL += "&redirect=" + url.QueryEscape(redirect)
	}

	http.Redirect(w, r, loginURL, http.StatusSeeOther)
}

// BootstrapAdmin creates the initial admin user from environment variables
// Only runs if no users exist in the database
func BootstrapAdmin(ctx context.Context, database *db.DB, allowedDomains []string) error {
	log := logger.Ctx(ctx)

	// Check if any users exist
	count, err := database.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		log.Info("Users exist, skipping admin bootstrap", "user_count", count)
		return nil
	}

	// Get bootstrap credentials from environment
	email := os.Getenv("ADMIN_BOOTSTRAP_EMAIL")
	password := os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")

	if email == "" || password == "" {
		return fmt.Errorf("ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD are required when no users exist")
	}

	// Validate email
	email = validation.NormalizeEmail(email)
	if !validation.IsValidEmail(email) {
		return fmt.Errorf("ADMIN_BOOTSTRAP_EMAIL is not a valid email address")
	}

	// Check email domain restriction
	if !validation.IsAllowedEmailDomain(email, allowedDomains) {
		parts := strings.SplitN(email, "@", 2)
		domain := ""
		if len(parts) == 2 {
			domain = parts[1]
		}
		return fmt.Errorf("ADMIN_BOOTSTRAP_EMAIL domain %q is not in ALLOWED_EMAIL_DOMAINS", domain)
	}

	// Validate password (minimum 8 characters)
	if len(password) < 8 {
		return fmt.Errorf("ADMIN_BOOTSTRAP_PASSWORD must be at least 8 characters")
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash bootstrap password: %w", err)
	}

	// Create admin user with password identity
	user, err := database.CreatePasswordUser(ctx, email, passwordHash, true /* isAdmin */)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	log.Warn("=== ADMIN USER CREATED ===",
		"email", email,
		"user_id", user.ID,
		"hint", "Change this password after first login")

	return nil
}
