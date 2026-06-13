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

// NewMiddleware returns HTTP middleware that extracts client IPs and:
// 1. Updates r.RemoteAddr to the primary (most trusted) IP
// 2. Stores Info in context for downstream use
//
// Proxy header priority (highest first):
//   - Fly-Client-IP: Set by Fly.io edge proxy, cannot be spoofed
//   - CF-Connecting-IP: Set by Cloudflare edge
//   - True-Client-IP: Akamai/Cloudflare Enterprise
//   - X-Real-IP: nginx reverse proxy
//   - X-Forwarded-For[0]: First hop (partially trusted)
//   - RemoteAddr: TCP connection (always available)
//
// trustedHeaders restricts which proxy headers are honored. Pass nil or empty
// to trust all known proxy headers (the default, suitable for deployments
// behind a header-stripping proxy). When set, only the listed headers are
// consulted for IP extraction — any header not in the list is ignored even if
// present, so an attacker cannot spoof an untrusted header to forge their IP.
// Header names are matched case-insensitively (HTTP-canonicalized).
func NewMiddleware(trustedHeaders []string) func(http.Handler) http.Handler {
	headerSet := makeHeaderSet(trustedHeaders) // nil means "trust all"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := extract(r, headerSet)

			// Update RemoteAddr for any downstream code that uses it directly
			r.RemoteAddr = info.Primary

			// Store in context for rate limiter, logger, etc.
			ctx := context.WithValue(r.Context(), clientIPKey, info)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// makeHeaderSet builds a lookup set of HTTP-canonicalized header names. It
// returns nil when no usable names are provided, which callers interpret as
// "trust all headers" (backward-compatible default).
func makeHeaderSet(headers []string) map[string]struct{} {
	set := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		if h = strings.TrimSpace(h); h != "" {
			set[http.CanonicalHeaderKey(h)] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
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

// proxyHeaders lists single-IP proxy headers in priority order (highest first).
// Each contains a single client IP set by an edge proxy or reverse proxy.
var proxyHeaders = []string{
	"Fly-Client-IP",    // Fly.io edge proxy
	"CF-Connecting-IP", // Cloudflare
	"True-Client-IP",   // Akamai/Cloudflare Enterprise
	"X-Real-IP",        // nginx reverse proxy
}

// extract pulls IPs from the trusted proxy headers and computes Primary +
// RateLimitKey. When trusted is nil, all known proxy headers are consulted
// (default). When non-nil, only headers whose canonical name is in the set are
// honored; others are ignored even if present.
func extract(r *http.Request, trusted map[string]struct{}) Info {
	allIPs := make(map[string]bool)

	remoteIP := extractIPFromAddr(r.RemoteAddr)
	if remoteIP != "" {
		allIPs[remoteIP] = true
	}

	isTrusted := func(header string) bool {
		if trusted == nil {
			return true
		}
		_, ok := trusted[http.CanonicalHeaderKey(header)]
		return ok
	}

	var primary string

	for _, header := range proxyHeaders {
		if !isTrusted(header) {
			continue
		}
		if ip := strings.TrimSpace(r.Header.Get(header)); ip != "" {
			allIPs[ip] = true
			if primary == "" {
				primary = ip
			}
		}
	}

	// X-Forwarded-For - only first IP is partially trusted
	if isTrusted("X-Forwarded-For") {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			if ip := strings.TrimSpace(strings.Split(forwarded, ",")[0]); ip != "" {
				allIPs[ip] = true
				if primary == "" {
					primary = ip
				}
			}
		}
	}

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
		return addr[:strings.LastIndex(addr, ":")]
	}

	// Plain IP (IPv4 or IPv6 without port)
	return addr
}
