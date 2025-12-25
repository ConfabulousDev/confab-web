package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractIPFromAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "IPv4 with port",
			addr:     "192.168.1.100:12345",
			expected: "192.168.1.100",
		},
		{
			name:     "IPv4 without port",
			addr:     "192.168.1.100",
			expected: "192.168.1.100",
		},
		{
			name:     "IPv6 with port",
			addr:     "[2001:db8::1]:8080",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 without port",
			addr:     "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 with brackets no port",
			addr:     "[2001:db8::1]",
			expected: "2001:db8::1",
		},
		{
			name:     "empty string",
			addr:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIPFromAddr(tt.addr)
			if result != tt.expected {
				t.Errorf("extractIPFromAddr(%q) = %q, want %q", tt.addr, result, tt.expected)
			}
		})
	}
}

func TestExtract_Primary(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "Fly-Client-IP takes highest priority",
			remoteAddr: "172.16.29.234:54686",
			headers: map[string]string{
				"Fly-Client-IP":     "203.0.113.45",
				"CF-Connecting-IP":  "198.51.100.1",
				"X-Real-IP":         "192.0.2.1",
				"X-Forwarded-For":   "10.0.0.1",
			},
			expected: "203.0.113.45",
		},
		{
			name:       "CF-Connecting-IP is second priority",
			remoteAddr: "172.16.29.234:54686",
			headers: map[string]string{
				"CF-Connecting-IP": "198.51.100.1",
				"X-Real-IP":        "192.0.2.1",
				"X-Forwarded-For":  "10.0.0.1",
			},
			expected: "198.51.100.1",
		},
		{
			name:       "True-Client-IP is third priority",
			remoteAddr: "172.16.29.234:54686",
			headers: map[string]string{
				"True-Client-IP":  "198.51.100.2",
				"X-Real-IP":       "192.0.2.1",
				"X-Forwarded-For": "10.0.0.1",
			},
			expected: "198.51.100.2",
		},
		{
			name:       "X-Real-IP is fourth priority",
			remoteAddr: "172.16.29.234:54686",
			headers: map[string]string{
				"X-Real-IP":       "192.0.2.1",
				"X-Forwarded-For": "10.0.0.1",
			},
			expected: "192.0.2.1",
		},
		{
			name:       "X-Forwarded-For first IP is fifth priority",
			remoteAddr: "172.16.29.234:54686",
			headers: map[string]string{
				"X-Forwarded-For": "10.0.0.1, 10.0.0.2, 10.0.0.3",
			},
			expected: "10.0.0.1",
		},
		{
			name:       "Falls back to RemoteAddr when no headers",
			remoteAddr: "192.168.1.100:12345",
			headers:    map[string]string{},
			expected:   "192.168.1.100",
		},
		{
			name:       "Trims whitespace from headers",
			remoteAddr: "172.16.0.1:8080",
			headers: map[string]string{
				"Fly-Client-IP": "  203.0.113.45  ",
			},
			expected: "203.0.113.45",
		},
		{
			name:       "Handles IPv6 in Fly-Client-IP",
			remoteAddr: "172.16.0.1:8080",
			headers: map[string]string{
				"Fly-Client-IP": "2001:db8::1",
			},
			expected: "2001:db8::1",
		},
		{
			name:       "Handles IPv6 RemoteAddr fallback",
			remoteAddr: "[2001:db8::1]:8080",
			headers:    map[string]string{},
			expected:   "2001:db8::1",
		},
		{
			name:       "Ignores empty Fly-Client-IP",
			remoteAddr: "192.168.1.100:12345",
			headers: map[string]string{
				"Fly-Client-IP": "",
				"X-Real-IP":     "192.0.2.1",
			},
			expected: "192.0.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			info := extract(req)
			if info.Primary != tt.expected {
				t.Errorf("extract().Primary = %q, want %q", info.Primary, tt.expected)
			}
		})
	}
}

func TestExtract_RateLimitKey(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "Combines all IPs sorted",
			remoteAddr: "172.16.29.234:54686",
			headers: map[string]string{
				"Fly-Client-IP":    "203.0.113.45",
				"CF-Connecting-IP": "198.51.100.1",
			},
			expected: "172.16.29.234|198.51.100.1|203.0.113.45",
		},
		{
			name:       "Deduplicates IPs",
			remoteAddr: "192.168.1.100:12345",
			headers: map[string]string{
				"Fly-Client-IP": "192.168.1.100", // Same as RemoteAddr
				"X-Real-IP":     "192.168.1.100", // Same again
			},
			expected: "192.168.1.100",
		},
		{
			name:       "Only RemoteAddr when no headers",
			remoteAddr: "192.168.1.100:12345",
			headers:    map[string]string{},
			expected:   "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For only uses first IP",
			remoteAddr: "172.16.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "10.0.0.1, 10.0.0.2, 10.0.0.3",
			},
			expected: "10.0.0.1|172.16.0.1",
		},
		{
			name:       "All headers combined",
			remoteAddr: "172.16.0.1:8080",
			headers: map[string]string{
				"Fly-Client-IP":    "1.1.1.1",
				"CF-Connecting-IP": "2.2.2.2",
				"True-Client-IP":   "3.3.3.3",
				"X-Real-IP":        "4.4.4.4",
				"X-Forwarded-For":  "5.5.5.5",
			},
			expected: "1.1.1.1|172.16.0.1|2.2.2.2|3.3.3.3|4.4.4.4|5.5.5.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			info := extract(req)
			if info.RateLimitKey != tt.expected {
				t.Errorf("extract().RateLimitKey = %q, want %q", info.RateLimitKey, tt.expected)
			}
		})
	}
}

func TestMiddleware_SetsRemoteAddr(t *testing.T) {
	var capturedRemoteAddr string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.16.29.234:54686"
	req.Header.Set("Fly-Client-IP", "203.0.113.45")

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if capturedRemoteAddr != "203.0.113.45" {
		t.Errorf("r.RemoteAddr = %q, want %q", capturedRemoteAddr, "203.0.113.45")
	}
}

func TestMiddleware_SetsContext(t *testing.T) {
	var capturedInfo Info

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInfo = FromRequest(r)
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.16.29.234:54686"
	req.Header.Set("Fly-Client-IP", "203.0.113.45")

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if capturedInfo.Primary != "203.0.113.45" {
		t.Errorf("FromRequest().Primary = %q, want %q", capturedInfo.Primary, "203.0.113.45")
	}

	expectedKey := "172.16.29.234|203.0.113.45"
	if capturedInfo.RateLimitKey != expectedKey {
		t.Errorf("FromRequest().RateLimitKey = %q, want %q", capturedInfo.RateLimitKey, expectedKey)
	}
}

func TestFromContext_ReturnsZeroWhenNotSet(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	info := FromRequest(req)

	if info.Primary != "" {
		t.Errorf("FromRequest().Primary = %q, want empty", info.Primary)
	}
	if info.RateLimitKey != "" {
		t.Errorf("FromRequest().RateLimitKey = %q, want empty", info.RateLimitKey)
	}
}
