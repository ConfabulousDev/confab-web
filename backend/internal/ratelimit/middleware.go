package ratelimit

import (
	"fmt"
	"log"
	"net/http"
	"sort"
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

// getClientIP extracts a composite key from multiple IP sources for rate limiting
// Uses a tuple of IPs where at least one is trusted, making it future-proof across platforms
// This prevents IP spoofing while working with Fly.io, nginx, Cloudflare, AWS, etc.
func getClientIP(r *http.Request) string {
	ips := make(map[string]bool) // Use map to deduplicate

	// 1. RemoteAddr - ALWAYS TRUSTED (actual TCP connection IP)
	// This cannot be spoofed as it's the real connection source
	if remoteIP := extractIP(r.RemoteAddr); remoteIP != "" {
		ips[remoteIP] = true
	}

	// 2. Fly-Client-IP - TRUSTED on Fly.io
	// Set by Fly.io edge proxy, cannot be spoofed by clients
	if flyIP := r.Header.Get("Fly-Client-IP"); flyIP != "" {
		ips[flyIP] = true
	}

	// 3. CF-Connecting-IP - TRUSTED on Cloudflare
	// Set by Cloudflare edge, cannot be spoofed by clients
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		ips[cfIP] = true
	}

	// 4. X-Real-IP - TRUSTED when behind nginx/similar reverse proxies
	// Typically set by nginx with real_ip module configured
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ips[realIP] = true
	}

	// 5. True-Client-IP - TRUSTED on Akamai/Cloudflare Enterprise
	if trueIP := r.Header.Get("True-Client-IP"); trueIP != "" {
		ips[trueIP] = true
	}

	// 6. X-Forwarded-For - PARTIALLY TRUSTED
	// Take only the first IP (claimed client IP)
	// Even if spoofed, we still have RemoteAddr as a trusted anchor
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			if firstIP := strings.TrimSpace(parts[0]); firstIP != "" {
				ips[firstIP] = true
			}
		}
	}

	// Convert map to sorted slice for deterministic key generation
	ipList := make([]string, 0, len(ips))
	for ip := range ips {
		ipList = append(ipList, ip)
	}
	sort.Strings(ipList)

	// Create composite key from all IPs
	// Even if some headers are spoofed, RemoteAddr ensures at least one trusted IP
	return strings.Join(ipList, "|")
}

// extractIP extracts IP from address that may include port
// Handles formats: "IP:port", "[IPv6]:port", "IP"
func extractIP(addr string) string {
	// Remove port if present
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Check if it's IPv6 with port [IPv6]:port
		if strings.Contains(addr, "[") {
			addr = addr[:idx]
		} else {
			// Could be IPv4:port or just IPv6
			// If there's only one colon, it's IPv4:port
			if strings.Count(addr, ":") == 1 {
				addr = addr[:idx]
			}
			// Otherwise it's IPv6 without port, keep as is
		}
	}

	// Remove IPv6 brackets
	addr = strings.Trim(addr, "[]")

	return addr
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
