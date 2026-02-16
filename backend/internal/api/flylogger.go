package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/go-chi/chi/v5/middleware"
)

// CLIUserAgent represents parsed CLI user agent information.
// User agent format: confab/1.2.3 (darwin; arm64)
type CLIUserAgent struct {
	Version string // e.g., "1.2.3"
	OS      string // e.g., "darwin", "linux", "windows"
	Arch    string // e.g., "arm64", "amd64"
}

// cliUserAgentRegex matches the CLI user agent format: confab/1.2.3 (darwin; arm64)
var cliUserAgentRegex = regexp.MustCompile(`^confab/([^\s]+)\s*\(([^;]+);\s*([^)]+)\)`)

// ParseCLIUserAgent extracts CLI version, OS, and architecture from the user agent string.
// Returns nil if the user agent doesn't match the expected CLI format.
func ParseCLIUserAgent(ua string) *CLIUserAgent {
	matches := cliUserAgentRegex.FindStringSubmatch(ua)
	if matches == nil {
		return nil
	}
	return &CLIUserAgent{
		Version: matches[1],
		OS:      matches[2],
		Arch:    matches[3],
	}
}

// Maximum length for error messages in logs
const maxErrorMessageLength = 200

// FlyLogger is a middleware that logs HTTP requests as structured JSON.
// Requires clientip.Middleware to run first to populate client IP in context.
//
// Log fields:
//   - method, path, proto: request info
//   - client_ip: real client IP (from Fly-Client-IP or RemoteAddr)
//   - status, bytes, duration_ms: response info
//   - req_id: request ID for tracing (from middleware.RequestID)
//   - user_id: authenticated user ID (when present)
//   - region, x_proto: Fly.io headers (when present)
//   - error: error message for 4xx responses (truncated, sanitized)
//   - user_agent: User-Agent header (truncated to 100 chars, sanitized)
func FlyLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging for health checks to reduce noise
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

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

		// Build structured log attributes
		attrs := []any{
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"proto", r.Proto,
			"client_ip", clientIP,
			"status", lrw.statusCode,
			"bytes", lrw.bytesWritten,
			"duration_ms", duration.Milliseconds(),
		}

		// Add request ID for tracing
		if reqID := middleware.GetReqID(r.Context()); reqID != "" {
			attrs = append(attrs, "req_id", reqID)
		}

		// Add user ID if authenticated
		if lrw.userIDSet {
			attrs = append(attrs, "user_id", lrw.userID)
		}

		// Add Fly.io info
		if region := r.Header.Get("Fly-Region"); region != "" {
			attrs = append(attrs, "region", sanitizeLogValue(region))
		}
		if flyProto := r.Header.Get("X-Forwarded-Proto"); flyProto != "" {
			attrs = append(attrs, "x_proto", sanitizeLogValue(flyProto))
		}

		// Add error message for 4xx responses
		if lrw.statusCode >= 400 && lrw.statusCode < 500 && len(lrw.body) > 0 {
			if errMsg := extractErrorMessage(lrw.body); errMsg != "" {
				attrs = append(attrs, "error", errMsg)
			}
		}

		// Add User-Agent - always log raw, plus parsed CLI fields when present
		if ua := r.Header.Get("User-Agent"); ua != "" {
			sanitized := sanitizeLogValue(ua)
			if runes := []rune(sanitized); len(runes) > 100 {
				sanitized = string(runes[:100]) + "..."
			}
			attrs = append(attrs, "user_agent", sanitized)

			// Also log structured CLI fields when user agent matches CLI format
			if cli := ParseCLIUserAgent(ua); cli != nil {
				attrs = append(attrs, "cli_version", cli.Version)
				attrs = append(attrs, "cli_os", cli.OS)
				attrs = append(attrs, "cli_arch", cli.Arch)
			}
		}

		logger.Info("http_request", attrs...)
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
