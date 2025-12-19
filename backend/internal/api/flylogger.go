package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Maximum length for error messages in logs
const maxErrorMessageLength = 200

// FlyLogger is a middleware that logs HTTP requests with Fly.io-aware client IP detection.
// It uses the Fly-Client-IP header (set by Fly's edge proxy) as the primary client IP,
// falling back to RemoteAddr for local development.
//
// Features:
//   - Logs real client IP (Fly-Client-IP or RemoteAddr)
//   - Logs authenticated user ID when present
//   - Logs error messages for 4xx responses (truncated if too long)
//   - Logs Fly.io region and protocol info when present
func FlyLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code, bytes written, and body for 4xx
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default if WriteHeader is never called
		}

		// Call the next handler
		next.ServeHTTP(lrw, r)

		// Calculate duration
		duration := time.Since(start)

		// Get real client IP (Fly-Client-IP or fallback to RemoteAddr)
		clientIP := getRealClientIP(r)

		// Build extra info parts
		var extraParts []string

		// Add user ID if authenticated (set by auth middleware via LogUserIDSetter)
		if lrw.userIDSet {
			extraParts = append(extraParts, "user="+strconv.FormatInt(lrw.userID, 10))
		}

		// Add Fly.io info (sanitize to prevent log injection)
		if region := r.Header.Get("Fly-Region"); region != "" {
			extraParts = append(extraParts, "region="+sanitizeLogValue(region))
		}
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			extraParts = append(extraParts, "proto="+sanitizeLogValue(proto))
		}

		// Add error message for 4xx responses (not 5xx - may contain sensitive info)
		if lrw.statusCode >= 400 && lrw.statusCode < 500 && len(lrw.body) > 0 {
			errMsg := extractErrorMessage(lrw.body)
			if errMsg != "" {
				extraParts = append(extraParts, "err="+errMsg)
			}
		}

		// Build log message
		extraInfo := ""
		if len(extraParts) > 0 {
			extraInfo = " [" + strings.Join(extraParts, " ") + "]"
		}

		log.Printf("\"%s %s %s\" from %s - %d %dB in %v%s",
			r.Method, r.URL.RequestURI(), r.Proto,
			clientIP,
			lrw.statusCode, lrw.bytesWritten, duration,
			extraInfo)
	})
}

// sanitizeLogValue removes characters that could enable log injection attacks.
// Replaces newlines, carriage returns, and other control characters with spaces.
func sanitizeLogValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r < 32 {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// extractErrorMessage extracts the error message from response body.
// Handles JSON format {"error": "message"} and plain text.
// Truncates long messages and sanitizes to prevent log injection.
func extractErrorMessage(body []byte) string {
	var msg string

	// Try to parse as JSON with "error" field
	var jsonErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &jsonErr); err == nil && jsonErr.Error != "" {
		msg = jsonErr.Error
	} else {
		// Use plain text body (trim whitespace/newlines)
		msg = strings.TrimSpace(string(body))
	}

	// Sanitize to prevent log injection
	msg = sanitizeLogValue(msg)

	// Truncate if too long (by runes to avoid splitting UTF-8)
	runes := []rune(msg)
	if len(runes) > maxErrorMessageLength {
		msg = string(runes[:maxErrorMessageLength]) + "..."
	}

	return msg
}

// getRealClientIP returns the real client IP address.
// It prioritizes Fly-Client-IP (set by Fly's edge proxy) over RemoteAddr.
func getRealClientIP(r *http.Request) string {
	// Fly-Client-IP is the canonical client IP on Fly.io
	// It's set by the edge proxy and cannot be spoofed by clients
	if flyIP := strings.TrimSpace(r.Header.Get("Fly-Client-IP")); flyIP != "" {
		return flyIP
	}

	// Fallback to RemoteAddr (for local development or non-Fly deployments)
	return extractIPFromAddr(r.RemoteAddr)
}

// extractIPFromAddr extracts the IP address from an address string.
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

// LogUserIDSetter is implemented by response writers that can capture the user ID for logging.
// Auth middleware should check for this interface and call SetLogUserID when a user is authenticated.
type LogUserIDSetter interface {
	SetLogUserID(id int64)
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code, bytes written,
// and response body for 4xx errors
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	body         []byte // Captured body for 4xx responses
	userID       int64  // User ID set by auth middleware
	userIDSet    bool   // Whether userID was set
}

// SetLogUserID implements LogUserIDSetter
func (lrw *loggingResponseWriter) SetLogUserID(id int64) {
	lrw.userID = id
	lrw.userIDSet = true
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	// Capture body for 4xx responses (for error logging)
	if lrw.statusCode >= 400 && lrw.statusCode < 500 {
		// Only capture up to maxErrorMessageLength + some buffer for JSON wrapper
		maxCapture := maxErrorMessageLength + 50
		if len(lrw.body) < maxCapture {
			remaining := maxCapture - len(lrw.body)
			if len(b) <= remaining {
				lrw.body = append(lrw.body, b...)
			} else {
				lrw.body = append(lrw.body, b[:remaining]...)
			}
		}
	}

	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytesWritten += n
	return n, err
}

// Flush implements http.Flusher for streaming responses (e.g., SSE)
func (lrw *loggingResponseWriter) Flush() {
	if f, ok := lrw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for middleware compatibility
func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}
