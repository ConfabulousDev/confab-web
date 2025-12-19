package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
)

func TestWithMaxBody(t *testing.T) {
	t.Run("allows request under limit", func(t *testing.T) {
		handler := withMaxBody(1024, func(w http.ResponseWriter, r *http.Request) {
			// Read the body to trigger MaxBytesReader
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			if err != nil {
				t.Errorf("unexpected error reading body: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		body := strings.Repeat("x", 512) // 512 bytes, under 1KB limit
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("rejects request over limit", func(t *testing.T) {
		handler := withMaxBody(1024, func(w http.ResponseWriter, r *http.Request) {
			// Read the body to trigger MaxBytesReader
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			if err != nil {
				// MaxBytesReader returns an error when limit exceeded
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		body := strings.Repeat("x", 2048) // 2KB, over 1KB limit
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413, got %d", rr.Code)
		}
	})

	t.Run("allows request at exact limit", func(t *testing.T) {
		handler := withMaxBody(1024, func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			if err != nil {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		body := strings.Repeat("x", 1024) // exactly 1KB
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("rejects request one byte over limit", func(t *testing.T) {
		handler := withMaxBody(1024, func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			if err != nil {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		body := strings.Repeat("x", 1025) // 1 byte over limit
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413, got %d", rr.Code)
		}
	})

	t.Run("empty body passes", func(t *testing.T) {
		handler := withMaxBody(1024, func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			if err != nil {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})
}

func TestMaxBodyConstants(t *testing.T) {
	// Verify constants are set correctly
	tests := []struct {
		name     string
		constant int64
		expected int64
	}{
		{"MaxBodyXS", MaxBodyXS, 2 * 1024},
		{"MaxBodyS", MaxBodyS, 16 * 1024},
		{"MaxBodyM", MaxBodyM, 128 * 1024},
		{"MaxBodyL", MaxBodyL, 2 * 1024 * 1024},
		{"MaxBodyXL", MaxBodyXL, 16 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.constant, tt.expected)
			}
		})
	}

	// Verify ordering: XS < S < M < L < XL
	if !(MaxBodyXS < MaxBodyS && MaxBodyS < MaxBodyM && MaxBodyM < MaxBodyL && MaxBodyL < MaxBodyXL) {
		t.Error("body size constants should be in ascending order: XS < S < M < L < XL")
	}
}

// TODO(2026-Q2): Remove this test when withMaxBodyGraced is removed
func TestWithMaxBodyGraced(t *testing.T) {
	// Helper to create request with user ID in context
	reqWithUserID := func(method, url string, body string, userID int64) *http.Request {
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.GetUserIDContextKey(), userID)
		return req.WithContext(ctx)
	}

	// Handler that reads body and returns appropriate status
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(r.Body)
		if err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	t.Run("applies normal limit for non-graced user", func(t *testing.T) {
		handler := withMaxBodyGraced(1024, 0, 9, testHandler)

		// User 10 should be subject to 1KB limit
		body := strings.Repeat("x", 2048) // 2KB, over limit
		req := reqWithUserID("POST", "/test", body, 10)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413 for non-graced user over limit, got %d", rr.Code)
		}
	})

	t.Run("applies graced limit for graced user", func(t *testing.T) {
		handler := withMaxBodyGraced(1024, 0, 9, testHandler)

		// User 9 should have no limit (gracedLimit=0)
		body := strings.Repeat("x", 2048) // 2KB, would be over normal limit
		req := reqWithUserID("POST", "/test", body, 9)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 for graced user, got %d", rr.Code)
		}
	})

	t.Run("applies normal limit when no user in context", func(t *testing.T) {
		handler := withMaxBodyGraced(1024, 0, 9, testHandler)

		// No user ID in context
		body := strings.Repeat("x", 2048)
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413 when no user in context, got %d", rr.Code)
		}
	})

	t.Run("non-graced user under limit passes", func(t *testing.T) {
		handler := withMaxBodyGraced(1024, 0, 9, testHandler)

		body := strings.Repeat("x", 512) // Under limit
		req := reqWithUserID("POST", "/test", body, 10)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 for non-graced user under limit, got %d", rr.Code)
		}
	})
}
