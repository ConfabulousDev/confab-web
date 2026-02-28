package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
)

func TestHandleReportClientErrors(t *testing.T) {
	handler := HandleReportClientErrors()

	// Helper to create a request with auth context
	makeRequest := func(body string, authenticated bool) (*httptest.ResponseRecorder, map[string]string) {
		req := httptest.NewRequest("POST", "/api/v1/client-errors", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), int64(42))
			req = req.WithContext(ctx)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		var resp map[string]string
		json.NewDecoder(rr.Body).Decode(&resp)
		return rr, resp
	}

	t.Run("valid payload with one error", func(t *testing.T) {
		body := `{
			"category": "transcript_validation",
			"session_id": "abc-123",
			"errors": [{
				"line": 42,
				"message_type": "assistant",
				"details": [{"path": "content.0.type", "message": "Invalid type"}],
				"raw_json_preview": "{\"type\":\"assistant\"}"
			}],
			"context": {"url": "/sessions/abc-123", "user_agent": "TestAgent/1.0"}
		}`
		rr, resp := makeRequest(body, true)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if resp["status"] != "ok" {
			t.Errorf("expected status ok, got %q", resp["status"])
		}
	})

	t.Run("valid payload with 50 errors", func(t *testing.T) {
		errors := make([]map[string]interface{}, 50)
		for i := range errors {
			errors[i] = map[string]interface{}{
				"line":    i + 1,
				"details": []map[string]string{{"path": "root", "message": "bad"}},
			}
		}
		payload := map[string]interface{}{
			"category": "transcript_validation",
			"errors":   errors,
		}
		body, _ := json.Marshal(payload)
		rr, _ := makeRequest(string(body), true)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("missing category", func(t *testing.T) {
		body := `{
			"errors": [{"line": 1, "details": [{"path": "root", "message": "bad"}]}]
		}`
		rr, resp := makeRequest(body, true)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		if !strings.Contains(resp["error"], "category") {
			t.Errorf("expected error about category, got %q", resp["error"])
		}
	})

	t.Run("empty errors array", func(t *testing.T) {
		body := `{"category": "transcript_validation", "errors": []}`
		rr, resp := makeRequest(body, true)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		if !strings.Contains(resp["error"], "empty") {
			t.Errorf("expected error about empty, got %q", resp["error"])
		}
	})

	t.Run("too many errors", func(t *testing.T) {
		errors := make([]map[string]interface{}, 51)
		for i := range errors {
			errors[i] = map[string]interface{}{
				"line":    i + 1,
				"details": []map[string]string{{"path": "root", "message": "bad"}},
			}
		}
		payload := map[string]interface{}{
			"category": "transcript_validation",
			"errors":   errors,
		}
		body, _ := json.Marshal(payload)
		rr, resp := makeRequest(string(body), true)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		if !strings.Contains(resp["error"], "max 50") {
			t.Errorf("expected error about max 50, got %q", resp["error"])
		}
	})

	t.Run("missing details in error", func(t *testing.T) {
		body := `{
			"category": "transcript_validation",
			"errors": [{"line": 1, "details": []}]
		}`
		rr, resp := makeRequest(body, true)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		if !strings.Contains(resp["error"], "details") {
			t.Errorf("expected error about details, got %q", resp["error"])
		}
	})

	t.Run("optional fields omitted", func(t *testing.T) {
		body := `{
			"category": "transcript_validation",
			"errors": [{"line": 1, "details": [{"path": "root", "message": "bad"}]}]
		}`
		rr, _ := makeRequest(body, true)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("unauthenticated still works (auth checked by middleware)", func(t *testing.T) {
		// The handler itself doesn't enforce auth — that's done by RequireSession middleware.
		// Without auth context, userID defaults to 0 in the log, but handler still processes.
		body := `{
			"category": "transcript_validation",
			"errors": [{"line": 1, "details": [{"path": "root", "message": "bad"}]}]
		}`
		rr, _ := makeRequest(body, false)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d (handler doesn't enforce auth itself)", rr.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		rr, resp := makeRequest("not json", true)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		if !strings.Contains(resp["error"], "Invalid request body") {
			t.Errorf("expected invalid body error, got %q", resp["error"])
		}
	})

	t.Run("multiple details per error", func(t *testing.T) {
		body := `{
			"category": "transcript_validation",
			"session_id": "abc-123",
			"errors": [{
				"line": 10,
				"message_type": "assistant",
				"details": [
					{"path": "content.0.type", "message": "Invalid type", "expected": "text", "received": "new_type"},
					{"path": "content.1.id", "message": "Required field missing"}
				],
				"raw_json_preview": "{\"type\":\"assistant\",\"content\":[{\"type\":\"new_type\"}]}"
			}]
		}`
		rr, resp := makeRequest(body, true)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if resp["status"] != "ok" {
			t.Errorf("expected status ok, got %q", resp["status"])
		}
	})
}
