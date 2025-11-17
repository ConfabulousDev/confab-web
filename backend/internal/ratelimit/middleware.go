package ratelimit

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Middleware creates an HTTP middleware that applies rate limiting
func Middleware(limiter RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			ip := getClientIP(r)

			// Check rate limit
			if !limiter.Allow(r.Context(), ip) {
				log.Printf("Rate limit exceeded for IP: %s, path: %s", ip, r.URL.Path)
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
				key = getClientIP(r)
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
		ip := getClientIP(r)

		if !limiter.Allow(r.Context(), ip) {
			log.Printf("Rate limit exceeded for IP: %s, path: %s", ip, r.URL.Path)
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		handler(w, r)
	}
}

// getClientIP extracts the client IP from the request
// Handles X-Forwarded-For and X-Real-IP headers for proxied requests
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by proxies/load balancers)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// Take the first one (client IP)
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (alternative to X-Forwarded-For)
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	// Format: "IP:port" or "[IPv6]:port"
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	// Remove IPv6 brackets
	ip = strings.Trim(ip, "[]")

	return ip
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

// EndpointKeyFunc combines IP and endpoint path for per-endpoint rate limiting
func EndpointKeyFunc(r *http.Request) string {
	ip := getClientIP(r)
	return fmt.Sprintf("%s:%s", ip, r.URL.Path)
}
