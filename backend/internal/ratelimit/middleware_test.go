package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/clientip"
)

// Note: IP extraction logic is now tested in internal/clientip package.
// These tests verify that the rate limiter correctly uses clientip.FromRequest.

func TestRateLimiter_UsesClientIPFromContext(t *testing.T) {
	tests := []struct {
		name             string
		remoteAddr       string
		headers          map[string]string
		expectedContains []string // IPs that should be in the composite key
		description      string
	}{
		{
			name:             "Direct connection - no proxy",
			remoteAddr:       "192.168.1.100:12345",
			headers:          map[string]string{},
			expectedContains: []string{"192.168.1.100"},
			description:      "Should use RemoteAddr only",
		},
		{
			name:       "Fly.io deployment",
			remoteAddr: "10.0.0.5:54321",
			headers: map[string]string{
				"Fly-Client-IP": "203.0.113.45",
			},
			expectedContains: []string{"10.0.0.5", "203.0.113.45"},
			description:      "Should include both RemoteAddr and Fly-Client-IP",
		},
		{
			name:       "Cloudflare deployment",
			remoteAddr: "172.16.0.10:8080",
			headers: map[string]string{
				"CF-Connecting-IP": "198.51.100.23",
				"X-Forwarded-For":  "198.51.100.23, 172.16.0.10",
			},
			expectedContains: []string{"172.16.0.10", "198.51.100.23"},
			description:      "Should include RemoteAddr, CF-Connecting-IP, and first X-Forwarded-For",
		},
		{
			name:       "Spoofing attempt - fake X-Forwarded-For",
			remoteAddr: "192.0.2.10:5000",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4",
				"Fly-Client-IP":   "192.0.2.10",
			},
			expectedContains: []string{"192.0.2.10", "1.2.3.4"},
			description:      "Should include spoofed IP but also trusted RemoteAddr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Simulate clientip.Middleware setting the context
			var capturedKey string
			handler := clientip.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedKey = clientip.FromRequest(r).RateLimitKey
			}))

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Check that all expected IPs are in the result
			for _, expectedIP := range tt.expectedContains {
				if !strings.Contains(capturedKey, expectedIP) {
					t.Errorf("%s: expected RateLimitKey to contain %q, but got %q",
						tt.description, expectedIP, capturedKey)
				}
			}

			t.Logf("%s: RateLimitKey = %q", tt.name, capturedKey)
		})
	}
}

func TestRateLimitKey_Deterministic(t *testing.T) {
	// Same request should always produce same key
	makeReq := func() *http.Request {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:8080"
		req.Header.Set("Fly-Client-IP", "203.0.113.50")
		req.Header.Set("X-Forwarded-For", "203.0.113.50, 192.168.1.1")
		return req
	}

	var key1, key2 string

	handler := clientip.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key1 == "" {
			key1 = clientip.FromRequest(r).RateLimitKey
		} else {
			key2 = clientip.FromRequest(r).RateLimitKey
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), makeReq())
	handler.ServeHTTP(httptest.NewRecorder(), makeReq())

	if key1 != key2 {
		t.Errorf("Same request should produce same key, got %q and %q", key1, key2)
	}
}

func TestRateLimitKey_DifferentRequests(t *testing.T) {
	// Different clients should produce different keys
	var key1, key2 string

	handler := clientip.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key1 == "" {
			key1 = clientip.FromRequest(r).RateLimitKey
		} else {
			key2 = clientip.FromRequest(r).RateLimitKey
		}
	}))

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:8080"
	req1.Header.Set("Fly-Client-IP", "203.0.113.50")

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:8080"
	req2.Header.Set("Fly-Client-IP", "203.0.113.51")

	handler.ServeHTTP(httptest.NewRecorder(), req1)
	handler.ServeHTTP(httptest.NewRecorder(), req2)

	if key1 == key2 {
		t.Errorf("Different requests should produce different keys, both got %q", key1)
	}
}
