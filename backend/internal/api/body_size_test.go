package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
