package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// TestDebugLoggingMiddleware tests that the debug logging middleware correctly
// preserves the full request body for downstream handlers
func TestDebugLoggingMiddleware(t *testing.T) {
	// Enable debug mode for this test, restore original state when done
	cleanup := logger.SetDebugForTest(true)
	defer cleanup()

	t.Run("preserves full request body for downstream handlers", func(t *testing.T) {
		// Create a large payload that exceeds maxDebugBodySize (10KB)
		// This is the critical test - the bug was that bodies > 10KB were truncated
		largeLines := make([]string, 500)
		for i := range largeLines {
			largeLines[i] = strings.Repeat("x", 100) // 100 chars per line
		}

		payload := map[string]interface{}{
			"session_id": "test-session-123",
			"file_name":  "transcript.jsonl",
			"file_type":  "transcript",
			"first_line": 1,
			"lines":      largeLines,
		}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}

		t.Logf("Test payload size: %d bytes (maxDebugBodySize is %d)", len(jsonPayload), maxDebugBodySize)
		if len(jsonPayload) <= maxDebugBodySize {
			t.Fatalf("test payload must be larger than maxDebugBodySize (%d), got %d", maxDebugBodySize, len(jsonPayload))
		}

		// Track what the downstream handler receives
		var receivedBody []byte
		var parseErr error

		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBody, _ = io.ReadAll(r.Body)
			// Try to parse as JSON to verify it's complete
			var parsed map[string]interface{}
			parseErr = json.Unmarshal(receivedBody, &parsed)
			w.WriteHeader(http.StatusOK)
		})

		// Wrap with debug logging middleware
		handler := debugLoggingMiddleware()(captureHandler)

		req := httptest.NewRequest("POST", "/api/v1/sync/chunk", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(jsonPayload))

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify the downstream handler received the FULL body
		if len(receivedBody) != len(jsonPayload) {
			t.Errorf("downstream handler received %d bytes, expected %d bytes", len(receivedBody), len(jsonPayload))
		}

		// Verify the body is valid JSON (not truncated causing parse errors)
		if parseErr != nil {
			t.Errorf("downstream handler failed to parse JSON: %v (this was the original bug - truncated body caused 'unexpected EOF')", parseErr)
		}

		// Verify content matches
		if !bytes.Equal(receivedBody, jsonPayload) {
			t.Error("downstream handler received different content than what was sent")
		}
	})

	t.Run("preserves small request body", func(t *testing.T) {
		payload := map[string]string{"msg": "hello"}
		jsonPayload, _ := json.Marshal(payload)

		var receivedBody []byte
		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		})

		handler := debugLoggingMiddleware()(captureHandler)

		req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(jsonPayload))

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !bytes.Equal(receivedBody, jsonPayload) {
			t.Errorf("body mismatch: got %q, want %q", string(receivedBody), string(jsonPayload))
		}
	})

	t.Run("handles empty body", func(t *testing.T) {
		var called bool
		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 {
				t.Errorf("expected empty body, got %d bytes", len(body))
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := debugLoggingMiddleware()(captureHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("handler was not called")
		}
	})

	t.Run("response capture works correctly", func(t *testing.T) {
		responseBody := `{"status":"ok","data":"test"}`

		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(responseBody))
		})

		handler := debugLoggingMiddleware()(echoHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if w.Body.String() != responseBody {
			t.Errorf("response body mismatch: got %q, want %q", w.Body.String(), responseBody)
		}
	})
}

// TestDebugLoggingMiddlewareDisabled verifies middleware is a no-op when debug is disabled
func TestDebugLoggingMiddlewareDisabled(t *testing.T) {
	// Disable debug mode for this test, restore original state when done
	cleanup := logger.SetDebugForTest(false)
	defer cleanup()

	// When debug is disabled, the middleware should pass through without reading body
	payload := []byte(`{"test": "data"}`)

	var receivedBody []byte
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	handler := debugLoggingMiddleware()(captureHandler)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(payload))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !bytes.Equal(receivedBody, payload) {
		t.Errorf("body mismatch when debug disabled: got %q, want %q", string(receivedBody), string(payload))
	}
}

// TestResponseCapture tests the responseCapture writer directly
func TestResponseCapture(t *testing.T) {
	t.Run("captures small response", func(t *testing.T) {
		w := httptest.NewRecorder()
		rc := &responseCapture{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			maxSize:        maxDebugBodySize,
		}

		data := []byte("small response")
		n, err := rc.Write(data)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if n != len(data) {
			t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
		}
		if rc.body.String() != string(data) {
			t.Errorf("captured body mismatch: got %q, want %q", rc.body.String(), string(data))
		}
		if rc.truncated {
			t.Error("small response should not be marked as truncated")
		}
	})

	t.Run("truncates large response in capture only", func(t *testing.T) {
		w := httptest.NewRecorder()
		rc := &responseCapture{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			maxSize:        100, // Small max for testing
		}

		// Write more than maxSize
		data := bytes.Repeat([]byte("x"), 200)
		n, err := rc.Write(data)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if n != len(data) {
			t.Errorf("expected to write %d bytes to underlying writer, wrote %d", len(data), n)
		}

		// Captured body should be truncated
		if rc.body.Len() != 100 {
			t.Errorf("captured body should be truncated to 100 bytes, got %d", rc.body.Len())
		}
		if !rc.truncated {
			t.Error("large response should be marked as truncated")
		}

		// But underlying response writer should have full content
		if w.Body.Len() != 200 {
			t.Errorf("underlying writer should have full 200 bytes, got %d", w.Body.Len())
		}
	})

	t.Run("captures status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		rc := &responseCapture{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			maxSize:        maxDebugBodySize,
		}

		rc.WriteHeader(http.StatusCreated)

		if rc.status != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rc.status)
		}
		if w.Code != http.StatusCreated {
			t.Errorf("underlying writer status: expected %d, got %d", http.StatusCreated, w.Code)
		}
	})
}
