package ratelimit

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
)

// Middleware creates an HTTP middleware that applies rate limiting
// Uses clientip.FromRequest for IP extraction (set by clientip.Middleware)
func Middleware(limiter RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get composite rate limit key from context (set by clientip.Middleware)
			key := clientip.FromRequest(r).RateLimitKey

			// Check rate limit
			if !limiter.Allow(r.Context(), key) {
				log.Printf("Rate limit exceeded for IP: %s, path: %s", key, r.URL.Path)
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareWithKey creates middleware that uses a custom key extractor
// Example: rate limit by user ID instead of IP
func MiddlewareWithKey(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get custom key
			key := keyFunc(r)
			if key == "" {
				// Fallback to IP if key function returns empty
				key = clientip.FromRequest(r).RateLimitKey
			}

			// Check rate limit
			if !limiter.Allow(r.Context(), key) {
				log.Printf("Rate limit exceeded for key: %s, path: %s", key, r.URL.Path)
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HandlerFunc wraps a single handler function with rate limiting
// Useful for applying different limits to specific endpoints
func HandlerFunc(limiter RateLimiter, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientip.FromRequest(r).RateLimitKey

		if !limiter.Allow(r.Context(), key) {
			log.Printf("Rate limit exceeded for IP: %s, path: %s", key, r.URL.Path)
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		handler(w, r)
	}
}

// UserKeyFunc extracts user ID from context for rate limiting by user
// Use with MiddlewareWithKey for authenticated endpoints
func UserKeyFunc(userIDKey interface{}) func(*http.Request) string {
	return func(r *http.Request) string {
		if userID, ok := r.Context().Value(userIDKey).(int64); ok {
			return fmt.Sprintf("user:%d", userID)
		}
		// Fallback to IP if no user ID in context
		return ""
	}
}
