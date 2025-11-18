package ratelimit

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetClientIP(t *testing.T) {
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
			name:       "Nginx deployment",
			remoteAddr: "10.1.0.50:9000",
			headers: map[string]string{
				"X-Real-IP":       "203.0.113.100",
				"X-Forwarded-For": "203.0.113.100, 10.1.0.50",
			},
			expectedContains: []string{"10.1.0.50", "203.0.113.100"},
			description:      "Should include RemoteAddr, X-Real-IP, and first X-Forwarded-For",
		},
		{
			name:       "Multiple proxies - all headers",
			remoteAddr: "10.0.0.1:443",
			headers: map[string]string{
				"Fly-Client-IP":    "203.0.113.50",
				"CF-Connecting-IP": "203.0.113.50",
				"X-Real-IP":        "203.0.113.50",
				"X-Forwarded-For":  "203.0.113.50, 172.16.0.1, 10.0.0.1",
			},
			expectedContains: []string{"10.0.0.1", "203.0.113.50"},
			description:      "Should deduplicate same IPs from different headers",
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
		{
			name:       "IPv6 connection",
			remoteAddr: "[2001:db8::1]:8080",
			headers: map[string]string{
				"Fly-Client-IP": "2001:db8::1",
			},
			expectedContains: []string{"2001:db8::1"},
			description:      "Should handle IPv6 addresses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := getClientIP(req)

			// Check that all expected IPs are in the result
			for _, expectedIP := range tt.expectedContains {
				if !strings.Contains(result, expectedIP) {
					t.Errorf("%s: expected result to contain %q, but got %q",
						tt.description, expectedIP, result)
				}
			}

			// Verify it's a pipe-delimited string
			if !strings.Contains(result, "|") && len(tt.expectedContains) > 1 {
				// If we expect multiple IPs, we should have a pipe
				parts := strings.Split(result, "|")
				if len(parts) < len(tt.expectedContains) {
					t.Errorf("%s: expected at least %d IPs in composite key, got %d (key: %q)",
						tt.description, len(tt.expectedContains), len(parts), result)
				}
			}

			t.Logf("%s: composite key = %q", tt.name, result)
		})
	}
}

func TestGetClientIP_Deterministic(t *testing.T) {
	// Same request should always produce same key
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:8080"
	req1.Header.Set("Fly-Client-IP", "203.0.113.50")
	req1.Header.Set("X-Forwarded-For", "203.0.113.50, 192.168.1.1")

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:8080"
	req2.Header.Set("Fly-Client-IP", "203.0.113.50")
	req2.Header.Set("X-Forwarded-For", "203.0.113.50, 192.168.1.1")

	key1 := getClientIP(req1)
	key2 := getClientIP(req2)

	if key1 != key2 {
		t.Errorf("Same request should produce same key, got %q and %q", key1, key2)
	}
}

func TestGetClientIP_DifferentRequests(t *testing.T) {
	// Different clients should produce different keys
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:8080"
	req1.Header.Set("Fly-Client-IP", "203.0.113.50")

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:8080"
	req2.Header.Set("Fly-Client-IP", "203.0.113.51")

	key1 := getClientIP(req1)
	key2 := getClientIP(req2)

	if key1 == key2 {
		t.Errorf("Different requests should produce different keys, both got %q", key1)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"192.168.1.1", "192.168.1.1"},
		{"[2001:db8::1]:8080", "2001:db8::1"},
		{"2001:db8::1", "2001:db8::1"},
		{"[::1]:80", "::1"},
		{"127.0.0.1:443", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractIP(tt.input)
			if result != tt.expected {
				t.Errorf("extractIP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
