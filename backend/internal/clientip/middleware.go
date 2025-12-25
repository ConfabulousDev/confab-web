// Package clientip provides middleware for extracting real client IPs
// in a platform-agnostic way (Fly.io, Cloudflare, nginx, etc.)
package clientip

import (
	"context"
	"net/http"
	"sort"
	"strings"
)

// contextKey is unexported to prevent collisions
type contextKey struct{}

var clientIPKey = contextKey{}

// Info contains extracted client IP information
type Info struct {
	// Primary is the most trusted single IP (for logging, display)
	// Priority: Fly-Client-IP > CF-Connecting-IP > True-Client-IP > X-Real-IP > XFF[0] > RemoteAddr
	Primary string

	// RateLimitKey is composite of all IPs for anti-spoofing
	// Even if some headers are spoofed, RemoteAddr anchors the key
	RateLimitKey string
}

// Middleware extracts client IPs from various headers and:
// 1. Updates r.RemoteAddr to the primary (most trusted) IP
// 2. Stores Info in context for downstream use
//
// Trusted header priority (highest first):
//   - Fly-Client-IP: Set by Fly.io edge proxy, cannot be spoofed
//   - CF-Connecting-IP: Set by Cloudflare edge
//   - True-Client-IP: Akamai/Cloudflare Enterprise
//   - X-Real-IP: nginx reverse proxy
//   - X-Forwarded-For[0]: First hop (partially trusted)
//   - RemoteAddr: TCP connection (always available)
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := extract(r)

		// Update RemoteAddr for any downstream code that uses it directly
		r.RemoteAddr = info.Primary

		// Store in context for rate limiter, logger, etc.
		ctx := context.WithValue(r.Context(), clientIPKey, info)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext retrieves Info from context
// Returns zero Info if not present (Primary and RateLimitKey will be empty)
func FromContext(ctx context.Context) Info {
	if info, ok := ctx.Value(clientIPKey).(Info); ok {
		return info
	}
	return Info{}
}

// FromRequest is a convenience wrapper around FromContext
func FromRequest(r *http.Request) Info {
	return FromContext(r.Context())
}

// extract pulls IPs from all known headers and computes Primary + RateLimitKey
func extract(r *http.Request) Info {
	// Collect all IPs for composite rate limit key
	allIPs := make(map[string]bool)

	// RemoteAddr - ALWAYS TRUSTED (actual TCP connection)
	remoteIP := extractIPFromAddr(r.RemoteAddr)
	if remoteIP != "" {
		allIPs[remoteIP] = true
	}

	// Primary IP - use first trusted header found (priority order)
	var primary string

	// 1. Fly-Client-IP - TRUSTED on Fly.io (set by edge proxy)
	if ip := strings.TrimSpace(r.Header.Get("Fly-Client-IP")); ip != "" {
		allIPs[ip] = true
		if primary == "" {
			primary = ip
		}
	}

	// 2. CF-Connecting-IP - TRUSTED on Cloudflare
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		allIPs[ip] = true
		if primary == "" {
			primary = ip
		}
	}

	// 3. True-Client-IP - TRUSTED on Akamai/Cloudflare Enterprise
	if ip := strings.TrimSpace(r.Header.Get("True-Client-IP")); ip != "" {
		allIPs[ip] = true
		if primary == "" {
			primary = ip
		}
	}

	// 4. X-Real-IP - TRUSTED when behind nginx/similar reverse proxies
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		allIPs[ip] = true
		if primary == "" {
			primary = ip
		}
	}

	// 5. X-Forwarded-For - PARTIALLY TRUSTED (first IP only)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				allIPs[ip] = true
				if primary == "" {
					primary = ip
				}
			}
		}
	}

	// 6. Fallback to RemoteAddr if no headers found
	if primary == "" {
		primary = remoteIP
	}

	// Build composite key from all IPs (sorted for determinism)
	ipList := make([]string, 0, len(allIPs))
	for ip := range allIPs {
		ipList = append(ipList, ip)
	}
	sort.Strings(ipList)
	rateLimitKey := strings.Join(ipList, "|")

	return Info{
		Primary:      primary,
		RateLimitKey: rateLimitKey,
	}
}

// extractIPFromAddr extracts IP from address that may include port
// Handles formats: "IP:port", "[IPv6]:port", "IP", "IPv6"
func extractIPFromAddr(addr string) string {
	if addr == "" {
		return ""
	}

	// Check for IPv6 with port [IPv6]:port
	if strings.HasPrefix(addr, "[") {
		if idx := strings.LastIndex(addr, "]:"); idx != -1 {
			return strings.Trim(addr[:idx+1], "[]")
		}
		// Just [IPv6] without port
		return strings.Trim(addr, "[]")
	}

	// Check for IPv4:port (exactly one colon)
	if strings.Count(addr, ":") == 1 {
		if idx := strings.LastIndex(addr, ":"); idx != -1 {
			return addr[:idx]
		}
	}

	// Plain IP (IPv4 or IPv6 without port)
	return addr
}
