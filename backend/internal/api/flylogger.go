package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/ConfabulousDev/confab-web/internal/clientip"
)

// Maximum length for error messages in logs
const maxErrorMessageLength = 200

// FlyLogger is a middleware that logs HTTP requests in a structured format.
// Requires clientip.Middleware to run first to populate client IP in context.
//
// Log format:
//
//	"METHOD /path HTTP/1.1" from IP - STATUS SIZEb in DURATION | key=value...
//
// Features:
//   - Logs real client IP (from clientip.Middleware context, or r.RemoteAddr fallback)
//   - Logs request ID for tracing (from middleware.RequestID)
//   - Logs authenticated user ID when present
//   - Logs Fly.io region and protocol info when present
//   - Logs error messages for 4xx responses (truncated, sanitized)
//   - Logs User-Agent (truncated to 100 chars, sanitized)
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

		// Get real client IP from context (set by clientip.Middleware)
		// Falls back to r.RemoteAddr if middleware hasn't run (shouldn't happen in prod)
		clientIP := clientip.FromRequest(r).Primary
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}

		// Build extra info parts
		var extraParts []string

		// Add request ID for tracing (set by middleware.RequestID)
		if reqID := middleware.GetReqID(r.Context()); reqID != "" {
			extraParts = append(extraParts, "req="+reqID)
		}

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

		// Add User-Agent at end (can be long, truncate to 100 runes)
		if ua := r.Header.Get("User-Agent"); ua != "" {
			ua = sanitizeLogValue(ua)
			// Truncate by runes to avoid splitting UTF-8 characters
			if runes := []rune(ua); len(runes) > 100 {
				ua = string(runes[:100]) + "..."
			}
			extraParts = append(extraParts, "ua="+ua)
		}

		// Build log message
		extraInfo := ""
		if len(extraParts) > 0 {
			extraInfo = " | " + strings.Join(extraParts, " ")
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
