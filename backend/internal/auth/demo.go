package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// CF-483: Demo identity support. Activated by DEMO_IDENTITY_EMAIL env var.
// When the env var is unset, every function in this file is a no-op or
// pass-through so demo mode imposes zero behavior change on regular
// deployments.

// DemoSessionExpiry is the lifetime stamped on the shared demo session
// row. Browsers cap cookie Max-Age at ~400 days regardless; the long
// DB expiry is so GetWebSession's expires_at > NOW() guard never trips.
const DemoSessionExpiry = 100 * 365 * 24 * time.Hour

// ReadOnlyUserError is the structured 403 body returned by EnforceReadOnly.
type ReadOnlyUserError struct {
	Error   string `json:"error"`   // always "read_only_user"
	Message string `json:"message"` // human-readable
}

// readOnlyKey carries the read-only flag through the request context
// after auth middleware resolves the user.
type readOnlyKey struct{}

// WithReadOnly returns ctx with the read-only flag stored.
func WithReadOnly(ctx context.Context, readOnly bool) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, readOnly)
}

// ReadOnlyFromContext reports whether the request's resolved user is
// read-only. Returns false when no user has been resolved.
func ReadOnlyFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(readOnlyKey{}).(bool)
	return v
}

// DemoSessionCookieID returns the deterministic web_sessions row ID for
// the shared demo cookie: base64url(HMAC-SHA256(csrfSecret, "demo:"+email)).
// Stable across replicas/restarts as long as inputs are stable.
//
// Returns "" when email is empty. That empty return is a sentinel:
// HandleLogout uses it to short-circuit the "is this the demo cookie?"
// check so non-demo deployments behave identically to today.
func DemoSessionCookieID(csrfSecret, email string) string {
	if email == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(csrfSecret))
	mac.Write([]byte("demo:" + email))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// BootstrapDemoIdentity idempotently provisions the demo user and its
// shared session row. Called from main.go after BootstrapAdmin when
// DEMO_IDENTITY_EMAIL is set. No-op when email is empty.
func BootstrapDemoIdentity(ctx context.Context, database *db.DB, email, csrfSecret string) error {
	if email == "" {
		return nil
	}

	log := logger.Ctx(ctx)
	userStore := &dbuser.Store{DB: database}
	authStore := &dbauth.Store{DB: database}

	user, preExisted, err := userStore.UpsertDemoIdentity(ctx, email)
	if err != nil {
		return fmt.Errorf("upsert demo identity: %w", err)
	}
	if preExisted {
		log.Warn("DEMO_IDENTITY_EMAIL matched a pre-existing user; flipped to read-only and stripped password",
			"email", email, "user_id", user.ID)
	} else {
		log.Info("demo identity provisioned", "email", email, "user_id", user.ID)
	}

	if err := userStore.DeletePasswordIdentitiesForUser(ctx, user.ID); err != nil {
		return fmt.Errorf("strip password identities from demo user: %w", err)
	}

	sharedID := DemoSessionCookieID(csrfSecret, email)
	expiresAt := time.Now().UTC().Add(DemoSessionExpiry)
	if err := authStore.UpsertSharedSession(ctx, sharedID, user.ID, expiresAt); err != nil {
		return fmt.Errorf("upsert shared demo session: %w", err)
	}

	pruned, err := authStore.DeleteOtherSessionsForUser(ctx, user.ID, sharedID)
	if err != nil {
		return fmt.Errorf("prune extra demo sessions: %w", err)
	}
	if pruned > 0 {
		log.Warn("pruned stale demo web_sessions rows", "user_id", user.ID, "pruned", pruned)
	}

	return nil
}

// AutoImpersonateIfDemo is the fallback called by RequireSession /
// RequireSessionOrAPIKey / OptionalAuth after real auth fails. When
// demoEmail is set, looks up the demo user, sets the shared cookie if
// the incoming request didn't carry it, and returns the resolved auth.
// Returns nil when demoEmail is empty (zero-behavior-change guard) or
// when the lookup fails.
func AutoImpersonateIfDemo(w http.ResponseWriter, r *http.Request, database *db.DB, demoEmail, csrfSecret string) *sessionAuthResult {
	if demoEmail == "" {
		return nil
	}

	ctx := r.Context()
	log := logger.Ctx(ctx)
	authStore := &dbauth.Store{DB: database}
	sharedID := DemoSessionCookieID(csrfSecret, demoEmail)

	session, err := authStore.GetWebSession(ctx, sharedID)
	if err != nil {
		// Shared row missing (operator skipped bootstrap, or the row
		// was deleted post-bootstrap). Re-provision lazily so the
		// fallback is self-healing.
		userStore := &dbuser.Store{DB: database}
		user, _, upsertErr := userStore.UpsertDemoIdentity(ctx, demoEmail)
		if upsertErr != nil {
			log.Warn("demo auto-impersonate: upsert failed", "error", upsertErr)
			return nil
		}
		expiresAt := time.Now().UTC().Add(DemoSessionExpiry)
		if err := authStore.UpsertSharedSession(ctx, sharedID, user.ID, expiresAt); err != nil {
			log.Warn("demo auto-impersonate: shared session upsert failed", "error", err)
			return nil
		}
		session = &models.WebSession{
			ID:         sharedID,
			UserID:     user.ID,
			UserEmail:  user.Email,
			UserStatus: user.Status,
			ReadOnly:   true,
		}
	}

	// Only set the cookie if the request didn't already carry it.
	if existing, err := r.Cookie(SessionCookieName); err != nil || existing.Value != sharedID {
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    sharedID,
			Path:     "/",
			Expires:  time.Now().UTC().Add(DemoSessionExpiry),
			HttpOnly: true,
			Secure:   cookieSecure(),
			SameSite: http.SameSiteLaxMode,
		})
	}

	return &sessionAuthResult{
		userID:       session.UserID,
		userEmail:    session.UserEmail,
		userReadOnly: session.ReadOnly,
	}
}

// EnforceReadOnly is HTTP middleware that returns the structured 403
// body for mutating requests when the resolved user has read_only=true.
// Pass-through when no user is in context or when read_only=false —
// safe-by-default for non-demo deployments.
//
// Chained internally by RequireAPIKey / RequireSession / RequireSessionOrAPIKey
// / OptionalAuth so it always runs AFTER user resolution (otherwise the
// context wouldn't have the read-only flag yet). Don't mount it
// standalone at a route group root — it must come after auth.
func EnforceReadOnly(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ReadOnlyFromContext(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			userID, _ := GetUserID(r.Context())
			logger.Ctx(r.Context()).Warn("read_only_user blocked mutating request",
				"user_id", userID,
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", clientip.FromRequest(r).Primary,
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(ReadOnlyUserError{
				Error:   "read_only_user",
				Message: "This identity is read-only.",
			})
		})
	}
}

// RenderDemoBannerScriptTag returns the <script> tag that exposes the
// demo identity email to the frontend via window.__DEMO_IDENTITY__.
// Returns "" when email is empty (so serveSPA can branch on the empty
// string instead of duplicating the demo-mode check). The email is
// JS-string-escaped via html/template so a misconfigured
// DEMO_IDENTITY_EMAIL cannot become an XSS vector even if startup
// validation is bypassed (defense in depth — IsValidEmail rejects
// payload-bearing emails on its own).
//
// html/template's JSEscapeString escapes < > & ' " and U+2028/2029,
// closing both the </script>-breakout and the JS-string-breakout
// vectors. text/template's same-named helper does NOT escape angle
// brackets — keep this import explicit.
func RenderDemoBannerScriptTag(email string) string {
	if email == "" {
		return ""
	}
	return fmt.Sprintf(`<script>window.__DEMO_IDENTITY__ = "%s";</script>`, htmltemplate.JSEscapeString(email))
}

// IsDemoLoginEmail reports whether the provided email matches the
// configured demo identity (case-insensitive). Used by HandlePasswordLogin
// and all OAuth callbacks to short-circuit before the credential check.
// Returns false when demoEmail is empty so non-demo deployments are
// unchanged.
func IsDemoLoginEmail(demoEmail, candidate string) bool {
	if demoEmail == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(demoEmail))
}

// redirectDemoLoginRejected sends the user back to /login with the
// generic access_denied message used when an OAuth callback resolves to
// the configured demo email. Centralized so the three OAuth callbacks
// stay identical and a future copy change touches one place. Caller
// must already have logged the rejection.
func redirectDemoLoginRejected(w http.ResponseWriter, r *http.Request, frontendURL string) {
	const message = "Your email domain is not permitted. Contact your administrator."
	errorURL := fmt.Sprintf("%s/login?error=access_denied&error_description=%s",
		frontendURL, url.QueryEscape(message))
	http.Redirect(w, r, errorURL, http.StatusTemporaryRedirect)
}
