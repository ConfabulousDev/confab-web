package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCsrfWhenSession tests the conditional CSRF middleware that applies CSRF
// protection only when the request uses session cookie auth (no Bearer token).
func TestCsrfWhenSession(t *testing.T) {
	// Mock CSRF middleware that sets a header to prove it ran
	mockCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-CSRF-Applied", "true")
			next.ServeHTTP(w, r)
		})
	}

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := csrfWhenSession(mockCSRF)(innerHandler)

	t.Run("applies CSRF when no Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/sessions/123/github-links", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-CSRF-Applied") != "true" {
			t.Error("expected CSRF middleware to be applied for request without Bearer token")
		}
	})

	t.Run("skips CSRF when Bearer token present", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/sessions/123/github-links", nil)
		req.Header.Set("Authorization", "Bearer cfb_test_key_12345")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-CSRF-Applied") == "true" {
			t.Error("expected CSRF middleware to be skipped for API key request")
		}
	})

	t.Run("applies CSRF for non-Bearer Authorization", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/test", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-CSRF-Applied") != "true" {
			t.Error("expected CSRF middleware to be applied for non-Bearer auth")
		}
	})
}
