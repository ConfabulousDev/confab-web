package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/validation"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const userIDContextKey contextKey = "userID"

// GetUserIDContextKey returns the context key for user ID
func GetUserIDContextKey() contextKey {
	return userIDContextKey
}

// GenerateAPIKey generates a new random API key with cfb_ prefix
// Returns both the raw key (to give to user) and the hash (to store in DB)
func GenerateAPIKey() (string, string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64 and add cfb_ prefix
	rawKey := "cfb_" + base64.URLEncoding.EncodeToString(bytes)[:40]

	// Hash the key for storage
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := fmt.Sprintf("%x", hash)

	return rawKey, keyHash, nil
}

// HashAPIKey hashes an API key for validation
func HashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", hash)
}

// enrichSpanWithUser adds user attributes to the current span for tracing
// Uses one-hot encoding for auth mode (exactly one of authAPIKey/authSession should be true)
func enrichSpanWithUser(ctx context.Context, userID int64, userEmail string, authAPIKey, authSession bool) {
	span := trace.SpanFromContext(ctx)
	attrs := []attribute.KeyValue{
		attribute.Int64("user.id", userID),
		attribute.String("user.email", userEmail),
	}
	if authAPIKey {
		attrs = append(attrs, attribute.Bool("auth.api_key", true))
	}
	if authSession {
		attrs = append(attrs, attribute.Bool("auth.session", true))
	}
	span.SetAttributes(attrs...)
}

// apiKeyAuthResult contains the result of API key authentication
type apiKeyAuthResult struct {
	userID    int64
	userEmail string
}

// TryAPIKeyAuth attempts to authenticate using an API key from the Authorization header.
// Returns the auth result if successful, nil otherwise.
// Does not reject - callers decide whether to require auth.
func TryAPIKeyAuth(r *http.Request, database *db.DB) *apiKeyAuthResult {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}

	// Expected format: "Bearer <api-key>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil
	}

	rawKey := parts[1]
	keyHash := HashAPIKey(rawKey)

	// Validate key in database
	userID, keyID, userEmail, userStatus, err := database.ValidateAPIKey(r.Context(), keyHash)
	if err != nil {
		log := logger.Ctx(r.Context())
		log.Warn("API key validation failed",
			"key_hash_prefix", keyHash[:8],
			"client_ip", clientip.FromRequest(r).Primary)
		return nil
	}

	// Check if user is inactive
	if userStatus == models.UserStatusInactive {
		log := logger.Ctx(r.Context())
		log.Warn("API key rejected: user inactive",
			"key_hash_prefix", keyHash[:8],
			"user_id", userID,
			"client_ip", clientip.FromRequest(r).Primary)
		return nil
	}

	// Update last used timestamp (fire and forget)
	go func() {
		if err := database.UpdateAPIKeyLastUsed(context.Background(), keyID); err != nil {
			logger.Warn("Failed to update API key last used", "error", err, "key_id", keyID)
		}
	}()

	return &apiKeyAuthResult{userID: userID, userEmail: userEmail}
}

// RequireAPIKey returns an HTTP middleware that requires API key authentication.
// If allowedDomains is non-empty, the user's email domain must match.
// Use TryAPIKeyAuth for optional authentication.
func RequireAPIKey(database *db.DB, allowedDomains []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Expected format: "Bearer <api-key>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			rawKey := parts[1]
			keyHash := HashAPIKey(rawKey)

			// Validate key in database
			userID, keyID, userEmail, userStatus, err := database.ValidateAPIKey(r.Context(), keyHash)
			if err != nil {
				log := logger.Ctx(r.Context())
				log.Warn("API key validation failed",
					"key_hash_prefix", keyHash[:8],
					"client_ip", clientip.FromRequest(r).Primary)
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Check if user is inactive
			if userStatus == models.UserStatusInactive {
				log := logger.Ctx(r.Context())
				log.Warn("API key rejected: user inactive",
					"key_hash_prefix", keyHash[:8],
					"user_id", userID,
					"client_ip", clientip.FromRequest(r).Primary)
				http.Error(w, "Account deactivated", http.StatusForbidden)
				return
			}

			// Check email domain restriction
			if !validation.IsAllowedEmailDomain(userEmail, allowedDomains) {
				http.Error(w, "Email domain not permitted", http.StatusForbidden)
				return
			}

			// Update last used timestamp (fire and forget)
			go func() {
				if err := database.UpdateAPIKeyLastUsed(context.Background(), keyID); err != nil {
					logger.Warn("Failed to update API key last used", "error", err, "key_id", keyID)
				}
			}()

			// Set user ID on logger's response writer
			setLogUserID(w, userID)

			// Enrich request-scoped logger with user_id
			log := logger.Ctx(r.Context()).With("user_id", userID)
			ctx := logger.WithLogger(r.Context(), log)

			// Enrich OpenTelemetry span with user info
			enrichSpanWithUser(ctx, userID, userEmail, true, false)

			// Add user ID to request context
			ctx = context.WithValue(ctx, userIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from request context
func GetUserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey).(int64)
	return userID, ok
}

// setLogUserID sets the user ID on the logger's response writer.
// It unwraps the ResponseWriter chain to find the LogUserIDSetter.
func setLogUserID(w http.ResponseWriter, userID int64) {
	for {
		if setter, ok := w.(interface{ SetLogUserID(int64) }); ok {
			setter.SetLogUserID(userID)
			return
		}
		// Try to unwrap
		if unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
			w = unwrapper.Unwrap()
		} else {
			return // No more wrappers, give up
		}
	}
}

// SetUserIDForTest is a test helper to set user ID in context.
// This allows tests to bypass middleware and directly set the authenticated user.
func SetUserIDForTest(ctx context.Context, userID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userIDContextKey, userID)
}
