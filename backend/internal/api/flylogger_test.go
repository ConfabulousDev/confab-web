package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// captureLogOutput captures logger output during a test function.
// Returns the captured log output as a string.
func captureLogOutput(t *testing.T, fn func()) string {
	t.Helper()

	// Create a pipe to capture output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Redirect logger output to the pipe
	cleanup := logger.SetOutputForTest(w)
	defer cleanup()

	// Run the test function
	fn()

	// Close write end and read all output
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	return buf.String()
}

// wrapWithClientIP wraps a handler with clientip.Middleware for tests
// This simulates the production middleware chain where clientip runs before FlyLogger
func wrapWithClientIP(h http.Handler) http.Handler {
	return clientip.Middleware(h)
}

func TestFlyLoggerMiddleware_LogsCorrectClientIP(t *testing.T) {
	// Create a simple handler that returns 200
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with clientip.Middleware (sets context) then FlyLogger (reads from context)
	// This matches the production middleware order
	wrapped := wrapWithClientIP(FlyLogger(handler))

	// Create request with Fly headers
	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	req.RemoteAddr = "172.16.29.234:54686" // Internal Fly IP
	req.Header.Set("Fly-Client-IP", "203.0.113.45")
	req.Header.Set("Fly-Region", "sjc")

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should log the real client IP (203.0.113.45), not the internal IP (172.16.29.234)
	if !strings.Contains(logOutput, `"client_ip":"203.0.113.45"`) {
		t.Errorf("Log should contain client_ip:203.0.113.45, got: %s", logOutput)
	}
	if strings.Contains(logOutput, "172.16.29.234") {
		t.Errorf("Log should NOT contain internal Fly IP 172.16.29.234, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsRegion(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.16.0.1:8080"
	req.Header.Set("Fly-Client-IP", "203.0.113.45")
	req.Header.Set("Fly-Region", "sjc")

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, `"region":"sjc"`) {
		t.Errorf("Log should contain region:sjc, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsProto(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.16.0.1:8080"
	req.Header.Set("Fly-Client-IP", "203.0.113.45")
	req.Header.Set("X-Forwarded-Proto", "https")

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, `"x_proto":"https"`) {
		t.Errorf("Log should contain x_proto:https, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/notfound", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, `"status":404`) {
		t.Errorf("Log should contain status:404, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsMethod(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/api/v1/sync/init", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, `"method":"POST"`) {
		t.Errorf("Log should contain method:POST, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions?limit=10", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, "/api/v1/sessions") {
		t.Errorf("Log should contain path /api/v1/sessions, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsDuration(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should contain duration_ms field
	if !strings.Contains(logOutput, `"duration_ms":`) {
		t.Errorf("Log should contain duration_ms field, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_NoFlyHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	// Local dev request - no Fly headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should log RemoteAddr IP when no Fly headers
	if !strings.Contains(logOutput, "127.0.0.1") {
		t.Errorf("Log should contain RemoteAddr IP 127.0.0.1 when no Fly headers, got: %s", logOutput)
	}
	// Should not have "region" field since no region header
	if strings.Contains(logOutput, `"region":`) {
		t.Errorf("Log should not contain region field when no Fly headers, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_CapturesResponseBytes(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!")) // 13 bytes
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should contain byte count
	if !strings.Contains(logOutput, `"bytes":13`) {
		t.Errorf("Log should contain bytes:13, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_ResponseWriterPassthrough(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Verify response passthrough
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}
	if rr.Header().Get("X-Custom-Header") != "test-value" {
		t.Errorf("Expected X-Custom-Header=test-value, got %s", rr.Header().Get("X-Custom-Header"))
	}
	if rr.Body.String() != "Created" {
		t.Errorf("Expected body 'Created', got %s", rr.Body.String())
	}
}

func TestResponseWriter_WriteWithoutExplicitStatus(t *testing.T) {
	// Handler that writes without calling WriteHeader (implicit 200)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should show 200 status (implicit)
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("Log should contain implicit status:200, got: %s", logOutput)
	}
}

// =============================================================================
// User ID Logging Tests (always log when authenticated)
// =============================================================================

func TestFlyLoggerMiddleware_LogsUserID_OnSuccess(t *testing.T) {
	// Simulate auth middleware setting user ID on the response writer
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if setter, ok := w.(interface{ SetLogUserID(int64) }); ok {
			setter.SetLogUserID(42)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, `"user_id":42`) {
		t.Errorf("Log should contain user_id:42 for authenticated request, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsUserID_On4xx(t *testing.T) {
	// Simulate auth middleware setting user ID, then handler returning error
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if setter, ok := w.(interface{ SetLogUserID(int64) }); ok {
			setter.SetLogUserID(123)
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/api/v1/sync/init", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, `"user_id":123`) {
		t.Errorf("Log should contain user_id:123 for authenticated 4xx request, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_NoUserID_WhenUnauthenticated(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	// Request without user ID in context
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if strings.Contains(logOutput, `"user_id":`) {
		t.Errorf("Log should NOT contain user_id for unauthenticated request, got: %s", logOutput)
	}
}

// =============================================================================
// Error Message Logging Tests (4xx only)
// =============================================================================

func TestFlyLoggerMiddleware_LogsErrorMessage_JSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid session ID"})
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions/bad", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, "Invalid session ID") {
		t.Errorf("Log should contain error message 'Invalid session ID', got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsErrorMessage_PlainText(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/api/v1/sync/chunk", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, "Rate limit exceeded") {
		t.Errorf("Log should contain error message 'Rate limit exceeded', got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_NoErrorMessage_On2xx(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should not contain "error" field for successful responses
	if strings.Contains(logOutput, `"error":`) {
		t.Errorf("Log should NOT contain error field for 2xx response, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_NoErrorMessage_On3xx(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new-location", http.StatusFound)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/old-location", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if strings.Contains(logOutput, `"error":`) {
		t.Errorf("Log should NOT contain error field for 3xx response, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsErrorMessage_401(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, "Missing Authorization header") {
		t.Errorf("Log should contain error message for 401, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsErrorMessage_403(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("DELETE", "/api/v1/sessions/123", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, "Access denied") {
		t.Errorf("Log should contain error message for 403, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsErrorMessage_404(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Session not found"})
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions/999", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	if !strings.Contains(logOutput, "Session not found") {
		t.Errorf("Log should contain error message for 404, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_TruncatesLongErrorMessage(t *testing.T) {
	// Create a very long error message (500+ chars)
	longMessage := strings.Repeat("x", 500)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, longMessage, http.StatusBadRequest)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/api/v1/sync/init", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should truncate and not contain the full 500-char message
	if strings.Contains(logOutput, longMessage) {
		t.Errorf("Log should truncate long error messages, got full message in: %s", logOutput)
	}
	// Should contain truncation indicator
	if !strings.Contains(logOutput, "...") {
		t.Errorf("Log should contain '...' for truncated messages, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_NoErrorMessage_On5xx(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database connection failed"})
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// 5xx errors should NOT log the error body (may contain sensitive info)
	if strings.Contains(logOutput, "Database connection failed") {
		t.Errorf("Log should NOT contain error body for 5xx responses, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_ResponsePassthrough_With4xxLogging(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "preserved")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"test error"}`))
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Verify response is still passed through correctly
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
	if rr.Header().Get("X-Custom-Header") != "preserved" {
		t.Errorf("Expected X-Custom-Header=preserved, got %s", rr.Header().Get("X-Custom-Header"))
	}
	if rr.Body.String() != `{"error":"test error"}` {
		t.Errorf("Expected body to be preserved, got %s", rr.Body.String())
	}
}

// =============================================================================
// Security Tests
// =============================================================================

func TestFlyLoggerMiddleware_SanitizesLogInjection(t *testing.T) {
	// Attacker tries to inject a fake log line via error message
	maliciousError := "bad request\n2025/01/01 00:00:00 \"GET /admin\" from 1.2.3.4 - 200 0B in 1ms [user=1]"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, maliciousError, http.StatusBadRequest)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/api/v1/sync/init", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// With JSON logging, the malicious content is safely escaped in the error field
	// The log should be valid JSON (single line)
	lines := strings.Split(strings.TrimSpace(logOutput), "\n")
	if len(lines) > 1 {
		t.Errorf("Log injection detected - multiple log lines created: %q", logOutput)
	}

	// Should contain error field with sanitized content
	if !strings.Contains(logOutput, `"error":`) {
		t.Errorf("Should contain error field: %q", logOutput)
	}

	// Verify the real request path is logged
	if !strings.Contains(logOutput, "/api/v1/sync/init") {
		t.Errorf("Should contain the real request path: %q", logOutput)
	}
}

func TestFlyLoggerMiddleware_SanitizesControlCharacters(t *testing.T) {
	// Error with various control characters
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error\twith\x00null\rand\ncontrol", http.StatusBadRequest)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should not contain raw control characters (they get sanitized before JSON encoding)
	if strings.ContainsAny(logOutput, "\x00\r") {
		t.Errorf("Control characters should be sanitized: %q", logOutput)
	}
}

func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"with\nnewline", "with newline"},
		{"with\rcarriage", "with carriage"},
		{"with\ttab", "with tab"}, // tabs (ASCII 9) are < 32 so sanitized
		{"with\x00null", "with null"},
		{"multi\n\rline", "multi  line"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeLogValue(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeLogValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// http.Flusher Support Tests
// =============================================================================

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushRecorder) Flush() {
	f.flushed = true
	f.ResponseRecorder.Flush()
}

func TestFlyLoggerMiddleware_SupportsFlush(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("chunk1"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		w.Write([]byte("chunk2"))
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/stream", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	rr := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	wrapped.ServeHTTP(rr, req)

	if !rr.flushed {
		t.Error("Flush should be called on underlying ResponseWriter")
	}
	if rr.Body.String() != "chunk1chunk2" {
		t.Errorf("Expected body 'chunk1chunk2', got %s", rr.Body.String())
	}
}

// =============================================================================
// CLI User Agent Parsing Tests
// =============================================================================

func TestParseCLIUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		expected *CLIUserAgent
	}{
		{
			name: "standard CLI user agent",
			ua:   "confab/1.2.3 (darwin; arm64)",
			expected: &CLIUserAgent{
				Version: "1.2.3",
				OS:      "darwin",
				Arch:    "arm64",
			},
		},
		{
			name: "linux amd64",
			ua:   "confab/0.1.0 (linux; amd64)",
			expected: &CLIUserAgent{
				Version: "0.1.0",
				OS:      "linux",
				Arch:    "amd64",
			},
		},
		{
			name: "windows x86",
			ua:   "confab/2.0.0-beta (windows; x86)",
			expected: &CLIUserAgent{
				Version: "2.0.0-beta",
				OS:      "windows",
				Arch:    "x86",
			},
		},
		{
			name: "version with build metadata",
			ua:   "confab/1.0.0+build.123 (darwin; arm64)",
			expected: &CLIUserAgent{
				Version: "1.0.0+build.123",
				OS:      "darwin",
				Arch:    "arm64",
			},
		},
		{
			name:     "browser user agent",
			ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			expected: nil,
		},
		{
			name:     "curl user agent",
			ua:       "curl/8.1.2",
			expected: nil,
		},
		{
			name:     "empty user agent",
			ua:       "",
			expected: nil,
		},
		{
			name:     "malformed - missing parens",
			ua:       "confab/1.0.0 darwin; arm64",
			expected: nil,
		},
		{
			name:     "malformed - missing semicolon",
			ua:       "confab/1.0.0 (darwin arm64)",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCLIUserAgent(tt.ua)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("ParseCLIUserAgent(%q) = %+v, want nil", tt.ua, result)
				}
			} else {
				if result == nil {
					t.Fatalf("ParseCLIUserAgent(%q) = nil, want %+v", tt.ua, tt.expected)
				}
				if result.Version != tt.expected.Version {
					t.Errorf("Version = %q, want %q", result.Version, tt.expected.Version)
				}
				if result.OS != tt.expected.OS {
					t.Errorf("OS = %q, want %q", result.OS, tt.expected.OS)
				}
				if result.Arch != tt.expected.Arch {
					t.Errorf("Arch = %q, want %q", result.Arch, tt.expected.Arch)
				}
			}
		})
	}
}

func TestFlyLoggerMiddleware_LogsCLIUserAgentFields(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("POST", "/api/v1/sync/init", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	req.Header.Set("User-Agent", "confab/1.2.3 (darwin; arm64)")

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should log raw user_agent
	if !strings.Contains(logOutput, `"user_agent":"confab/1.2.3 (darwin; arm64)"`) {
		t.Errorf("Log should contain user_agent, got: %s", logOutput)
	}
	// Should ALSO log structured CLI fields
	if !strings.Contains(logOutput, `"cli_version":"1.2.3"`) {
		t.Errorf("Log should contain cli_version:1.2.3, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"cli_os":"darwin"`) {
		t.Errorf("Log should contain cli_os:darwin, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"cli_arch":"arm64"`) {
		t.Errorf("Log should contain cli_arch:arm64, got: %s", logOutput)
	}
}

func TestFlyLoggerMiddleware_LogsRawUserAgent_ForNonCLI(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := FlyLogger(handler)

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	logOutput := captureLogOutput(t, func() {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
	})

	// Should log raw user_agent for non-CLI requests
	if !strings.Contains(logOutput, `"user_agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"`) {
		t.Errorf("Log should contain user_agent for non-CLI request, got: %s", logOutput)
	}
	// Should NOT log CLI fields
	if strings.Contains(logOutput, `"cli_version":`) {
		t.Errorf("Log should NOT contain cli_version for non-CLI request, got: %s", logOutput)
	}
}
