package auth

import "testing"

// TestIsLocalhostURL tests the URL validation for CLI callback URLs
// This is security-critical: prevents open redirect attacks
func TestIsLocalhostURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		// Valid localhost URLs
		{"valid localhost with port", "http://localhost:8080/callback", true},
		{"valid localhost without port", "http://localhost/callback", true},
		{"valid 127.0.0.1 with port", "http://127.0.0.1:3000/callback", true},
		{"valid 127.0.0.1 without port", "http://127.0.0.1/callback", true},
		{"valid localhost root path", "http://localhost:8080/", true},
		{"valid localhost empty path", "http://localhost:8080", true},
		{"valid localhost with query params", "http://localhost:8080/callback?foo=bar", true},

		// Invalid: empty/malformed
		{"empty string", "", false},
		{"just text", "localhost", false},
		{"missing scheme", "localhost:8080/callback", false},
		{"invalid URL", "not-a-url", false},

		// Invalid: wrong scheme (security: must be http for localhost)
		{"https not allowed", "https://localhost:8080/callback", false},
		{"ftp not allowed", "ftp://localhost:8080/callback", false},
		{"file not allowed", "file://localhost/path", false},

		// Invalid: not localhost (security: prevents redirect to external sites)
		{"external domain", "http://example.com/callback", false},
		{"external IP", "http://192.168.1.1:8080/callback", false},
		{"external with localhost in path", "http://evil.com/localhost", false},
		{"localhost as subdomain", "http://localhost.evil.com/callback", false},

		// Invalid: credential injection attacks (security: prevents user@host bypass)
		{"credentials in URL", "http://user:pass@localhost:8080/callback", false},
		{"username only", "http://user@localhost:8080/callback", false},
		{"localhost as username", "http://localhost@evil.com/callback", false},

		// Invalid: port validation
		{"port zero", "http://localhost:0/callback", false},
		{"port too high", "http://localhost:65536/callback", false},
		{"negative port (invalid URL)", "http://localhost:-1/callback", false},
		{"non-numeric port", "http://localhost:abc/callback", false},

		// Edge cases
		{"IPv6 localhost not supported", "http://[::1]:8080/callback", false},
		{"localhost with trailing dot", "http://localhost.:8080/callback", false},
		{"case sensitivity - uppercase", "http://LOCALHOST:8080/callback", false},
		{"case sensitivity - mixed", "http://LocalHost:8080/callback", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalhostURL(tt.url)
			if result != tt.expected {
				t.Errorf("isLocalhostURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

// TestIsLocalhostURL_OpenRedirectPrevention specifically tests attack vectors
func TestIsLocalhostURL_OpenRedirectPrevention(t *testing.T) {
	// These are common open redirect attack patterns
	attacks := []string{
		// Basic external redirects
		"http://evil.com",
		"http://attacker.com/steal?token=",

		// Credential-based attacks (user info in URL)
		"http://localhost@evil.com",
		"http://localhost:password@evil.com",

		// URL parsing tricks
		"http://evil.com#localhost",
		"http://evil.com?localhost",
		"http://evil.com/localhost",

		// Scheme tricks
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",

		// Protocol-relative URLs
		"//evil.com",

		// Backslash tricks (some parsers treat \ as /)
		"http://localhost\\@evil.com",
	}

	for _, attack := range attacks {
		t.Run(attack, func(t *testing.T) {
			if isLocalhostURL(attack) {
				t.Errorf("isLocalhostURL(%q) = true, should reject this attack vector", attack)
			}
		})
	}
}
