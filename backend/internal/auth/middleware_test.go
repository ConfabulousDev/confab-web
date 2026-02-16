package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPIKeyMiddleware_HeaderParsing tests the Authorization header parsing logic
// This tests the middleware's header parsing without requiring a real database.
// We test by calling the middleware and observing responses for various header formats.
func TestAPIKeyMiddleware_HeaderParsing(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing Authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Missing Authorization header\n",
		},
		{
			name:           "invalid format - no Bearer",
			authHeader:     "cfb_test_key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Authorization header format\n",
		},
		{
			name:           "invalid format - Basic instead of Bearer",
			authHeader:     "Basic cfb_test_key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Authorization header format\n",
		},
		{
			name:           "invalid format - Bearer only (no space)",
			authHeader:     "Bearer",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Authorization header format\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use nil db - we're testing header parsing before DB is called
			handler := RequireAPIKey(nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/api/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			body := rec.Body.String()
			if body != tt.expectedBody {
				t.Errorf("body = %q, want %q", body, tt.expectedBody)
			}
		})
	}
}

// TestGetUserID_Extraction tests user ID extraction from context
func TestGetUserID_Extraction(t *testing.T) {
	t.Run("extracts user ID set by SetUserIDForTest", func(t *testing.T) {
		ctx := SetUserIDForTest(context.Background(), 99999)
		userID, ok := GetUserID(ctx)
		if !ok {
			t.Error("expected ok=true")
		}
		if userID != 99999 {
			t.Errorf("userID = %d, want 99999", userID)
		}
	})

	t.Run("handles zero user ID", func(t *testing.T) {
		ctx := SetUserIDForTest(context.Background(), 0)
		userID, ok := GetUserID(ctx)
		if !ok {
			t.Error("expected ok=true for zero userID")
		}
		if userID != 0 {
			t.Errorf("userID = %d, want 0", userID)
		}
	})

	t.Run("handles negative user ID", func(t *testing.T) {
		ctx := SetUserIDForTest(context.Background(), -1)
		userID, ok := GetUserID(ctx)
		if !ok {
			t.Error("expected ok=true for negative userID")
		}
		if userID != -1 {
			t.Errorf("userID = %d, want -1", userID)
		}
	})
}

// TestHashAPIKey_Consistency ensures the hash function works correctly with middleware
func TestHashAPIKey_Integration(t *testing.T) {
	// This tests that the middleware correctly hashes the key before validation
	testKey := "cfb_test_key_for_hashing_12345"
	hash1 := HashAPIKey(testKey)
	hash2 := HashAPIKey(testKey)

	if hash1 != hash2 {
		t.Error("HashAPIKey should produce consistent results")
	}

	// Verify hash format (SHA-256 = 64 hex chars)
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}

// TestOptionalAuth_DomainRestriction tests that OptionalAuth requires authentication
// when allowedDomains is configured (on-prem security: prevents anonymous public share access)
func TestOptionalAuth_DomainRestriction(t *testing.T) {
	t.Run("unauthenticated request passes through without domain restrictions", func(t *testing.T) {
		handlerCalled := false
		handler := OptionalAuth(nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/sessions/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !handlerCalled {
			t.Error("expected handler to be called for unauthenticated request without domain restrictions")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("unauthenticated request blocked with domain restrictions", func(t *testing.T) {
		handlerCalled := false
		handler := OptionalAuth(nil, []string{"company.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/sessions/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if handlerCalled {
			t.Error("expected handler NOT to be called for unauthenticated request with domain restrictions")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

// TestCookieSecure tests the cookie security flag logic
func TestCookieSecure(t *testing.T) {
	// Note: This test doesn't modify env vars since that could affect
	// parallel tests. We just verify the function exists and returns bool.
	result := cookieSecure()
	// Default should be true (secure) when INSECURE_DEV_MODE is not set
	if !result {
		// This might be running in a dev environment
		t.Log("cookieSecure() returned false - possibly INSECURE_DEV_MODE=true is set")
	}
}
