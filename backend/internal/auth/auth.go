package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/santaclaude2025/confab/backend/internal/db"
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

// GenerateAndHashAPIKey is an alias for GenerateAPIKey for clarity
func GenerateAndHashAPIKey() (string, string, error) {
	return GenerateAPIKey()
}

// HashAPIKey hashes an API key for validation
func HashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", hash)
}

// Middleware returns an HTTP middleware that validates API keys
func Middleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
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
			userID, keyID, err := database.ValidateAPIKey(r.Context(), keyHash)
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Update last used timestamp (fire and forget - don't block the request)
			go func() {
				_ = database.UpdateAPIKeyLastUsed(context.Background(), keyID)
			}()

			// Add user ID to request context
			ctx := context.WithValue(r.Context(), userIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from request context
func GetUserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey).(int64)
	return userID, ok
}
