package redactor

import (
	"regexp"
	"testing"
)

// TestAnthropicAPIKeyPattern tests the Anthropic API key pattern
func TestAnthropicAPIKeyPattern(t *testing.T) {
	pattern := `sk-ant-api\d{2}-[A-Za-z0-9_-]{95}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validKeys := []string{
		"sk-ant-api03-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-1234567890abcdefghijklm",
		"sk-ant-api01-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789_-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789_-aBcDeFgHiJkLmNoPqRs",
	}

	for _, key := range validKeys {
		if !re.MatchString(key) {
			t.Errorf("Pattern should match valid Anthropic API key: %s", key)
		}
	}

	// False positives - should NOT match
	invalidKeys := []string{
		"sk-ant-api-short",                    // Too short
		"sk-ant-api99-",                       // Missing characters
		"sk-ant-apix3-" + string(make([]byte, 95)), // Invalid format
		"sk-openai-1234567890",                // Different API key format
		"not-an-api-key",                      // Random string
		"sk-ant-api",                          // Incomplete
	}

	for _, key := range invalidKeys {
		if re.MatchString(key) {
			t.Errorf("Pattern should NOT match invalid key: %s", key)
		}
	}
}

// TestOpenAIAPIKeyPattern tests the OpenAI API key pattern
func TestOpenAIAPIKeyPattern(t *testing.T) {
	pattern := `sk-[A-Za-z0-9]{48}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validKeys := []string{
		"sk-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKL",
		"sk-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789aBcDeFgHiJkL",
	}

	for _, key := range validKeys {
		if !re.MatchString(key) {
			t.Errorf("Pattern should match valid OpenAI API key: %s", key)
		}
	}

	// False positives - should NOT match
	invalidKeys := []string{
		"sk-short",                            // Too short
		"sk-" + string(make([]byte, 40)),      // Wrong length
		"sk-1234567890abcdefghij!@#$%^&*()",  // Invalid characters
		"not-sk-prefix",                       // Wrong prefix
	}

	for _, key := range invalidKeys {
		if re.MatchString(key) {
			t.Errorf("Pattern should NOT match invalid key: %s", key)
		}
	}
}

// TestAWSAccessKeyPattern tests the AWS access key pattern
func TestAWSAccessKeyPattern(t *testing.T) {
	pattern := `AKIA[0-9A-Z]{16}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validKeys := []string{
		"AKIAIOSFODNN7EXAMPLE",
		"AKIABCDEFGHIJKLMNOPQ",
		"AKIA1234567890ABCDEF",
	}

	for _, key := range validKeys {
		if !re.MatchString(key) {
			t.Errorf("Pattern should match valid AWS access key: %s", key)
		}
	}

	// False positives - should NOT match
	invalidKeys := []string{
		"AKIA123",              // Too short
		"AKIAIOSFODNN7example", // Lowercase not allowed
		"BKIAIOSFODNN7EXAMPLE", // Wrong prefix
		"AKIAabcdefghijklmnop", // Lowercase letters
	}

	for _, key := range invalidKeys {
		if re.MatchString(key) {
			t.Errorf("Pattern should NOT match invalid key: %s", key)
		}
	}
}

// TestGitHubTokenPattern tests the GitHub personal access token pattern
func TestGitHubTokenPattern(t *testing.T) {
	pattern := `ghp_[A-Za-z0-9]{36}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validTokens := []string{
		"ghp_1234567890abcdefghijklmnopqrstuvwxyz",
		"ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
	}

	for _, token := range validTokens {
		if !re.MatchString(token) {
			t.Errorf("Pattern should match valid GitHub token: %s", token)
		}
	}

	// False positives - should NOT match
	invalidTokens := []string{
		"ghp_short",           // Too short
		"gho_" + string(make([]byte, 36)), // Wrong prefix
		"ghp_12345",           // Too short
		"not-github-token",    // No prefix
	}

	for _, token := range invalidTokens {
		if re.MatchString(token) {
			t.Errorf("Pattern should NOT match invalid token: %s", token)
		}
	}
}

// TestJWTPattern tests the JWT token pattern
func TestJWTPattern(t *testing.T) {
	pattern := `eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validJWTs := []string{
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		"eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJodHRwczovL2V4YW1wbGUuY29tIn0.signature",
	}

	for _, jwt := range validJWTs {
		if !re.MatchString(jwt) {
			t.Errorf("Pattern should match valid JWT: %s", jwt)
		}
	}

	// False positives - should NOT match
	invalidJWTs := []string{
		"not.a.jwt",           // No eyJ prefix
		"eyJ.eyJ.",            // Incomplete
		"eyJtest",             // No dots
		"random string",       // Random text
	}

	for _, jwt := range invalidJWTs {
		if re.MatchString(jwt) {
			t.Errorf("Pattern should NOT match invalid JWT: %s", jwt)
		}
	}
}

// TestPrivateKeyPattern tests the private key pattern
func TestPrivateKeyPattern(t *testing.T) {
	pattern := `-----BEGIN (RSA|EC|OPENSSH) PRIVATE KEY-----`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validHeaders := []string{
		"-----BEGIN RSA PRIVATE KEY-----",
		"-----BEGIN EC PRIVATE KEY-----",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
	}

	for _, header := range validHeaders {
		if !re.MatchString(header) {
			t.Errorf("Pattern should match valid private key header: %s", header)
		}
	}

	// False positives - should NOT match
	invalidHeaders := []string{
		"-----BEGIN PUBLIC KEY-----",      // Public key, not private
		"-----BEGIN CERTIFICATE-----",     // Certificate, not key
		"-----BEGIN PRIVATE KEY-----",     // Generic, not specific type
		"BEGIN RSA PRIVATE KEY",           // Missing dashes
	}

	for _, header := range invalidHeaders {
		if re.MatchString(header) {
			t.Errorf("Pattern should NOT match invalid header: %s", header)
		}
	}
}

// TestPostgreSQLConnectionStringPattern tests the PostgreSQL connection string pattern
func TestPostgreSQLConnectionStringPattern(t *testing.T) {
	pattern := `postgres(?:ql)?://[^:]+:([^@\s]+)@`
	re := regexp.MustCompile(pattern)

	// True positives - should match and capture password
	validConnStrings := []string{
		"postgres://user:password123@localhost:5432/db",
		"postgresql://admin:secret@db.example.com/mydb",
		"postgres://dbuser:my-p@ssw0rd@127.0.0.1:5432/",
	}

	for _, connStr := range validConnStrings {
		matches := re.FindStringSubmatch(connStr)
		if matches == nil {
			t.Errorf("Pattern should match valid PostgreSQL connection string: %s", connStr)
		}
		if len(matches) < 2 {
			t.Errorf("Pattern should capture password from connection string: %s", connStr)
		}
	}

	// False positives - should NOT match
	invalidConnStrings := []string{
		"mysql://user:password@localhost/db",  // Different database
		"postgres://localhost:5432/db",         // No password
		"http://user:pass@example.com",         // Not a database connection
	}

	for _, connStr := range invalidConnStrings {
		if re.MatchString(connStr) {
			t.Errorf("Pattern should NOT match non-PostgreSQL string: %s", connStr)
		}
	}
}

// TestGenericPasswordPattern tests a generic password pattern for URLs
func TestGenericPasswordPattern(t *testing.T) {
	pattern := `://[^:/@\s]+:([^@\s]+)@`
	re := regexp.MustCompile(pattern)

	// True positives - should match and capture password
	validURLs := []string{
		"https://user:password@example.com",
		"ftp://admin:secret123@ftp.example.com",
		"redis://default:mypassword@redis.local:6379",
	}

	for _, url := range validURLs {
		matches := re.FindStringSubmatch(url)
		if matches == nil {
			t.Errorf("Pattern should match URL with credentials: %s", url)
		}
		if len(matches) < 2 {
			t.Errorf("Pattern should capture password from URL: %s", url)
		}
	}

	// False positives - should NOT match
	invalidURLs := []string{
		"https://example.com",              // No credentials
		"user:password",                    // No URL format
		"://no-user@example.com",          // No user
	}

	for _, url := range invalidURLs {
		if re.MatchString(url) {
			t.Errorf("Pattern should NOT match URL without proper credentials: %s", url)
		}
	}
}

// TestSlackTokenPattern tests the Slack token patterns
func TestSlackTokenPattern(t *testing.T) {
	pattern := `xox[baprs]-[0-9a-zA-Z-]{10,72}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validTokens := []string{
		"xoxb-1234567890-1234567890-abcdefghijklmnopqrstuvwx",
		"xoxp-1234567890-1234567890-1234567890-abc",
		"xoxa-1234567890",
		"xoxr-abcdefghij",
		"xoxs-1234567890-1234567890-1234567890-abcdefghijklmnopqrstuvwxyz",
	}

	for _, token := range validTokens {
		if !re.MatchString(token) {
			t.Errorf("Pattern should match valid Slack token: %s", token)
		}
	}

	// False positives - should NOT match
	invalidTokens := []string{
		"xoxb-short",          // Too short
		"xoxx-1234567890",     // Invalid type
		"not-slack-token",     // No prefix
	}

	for _, token := range invalidTokens {
		if re.MatchString(token) {
			t.Errorf("Pattern should NOT match invalid token: %s", token)
		}
	}
}

// TestStripeAPIKeyPattern tests the Stripe API key pattern
func TestStripeAPIKeyPattern(t *testing.T) {
	pattern := `sk_live_[0-9a-zA-Z]{24,}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validKeys := []string{
		"sk_live_1234567890abcdefghijklmnopqrstuvwxyz",
		"sk_live_aBcDeFgHiJkLmNoPqRsTuVwXyZ",
	}

	for _, key := range validKeys {
		if !re.MatchString(key) {
			t.Errorf("Pattern should match valid Stripe API key: %s", key)
		}
	}

	// False positives - should NOT match
	invalidKeys := []string{
		"sk_test_1234567890abcdefghij", // Test key, not live
		"sk_live_short",                 // Too short
		"pk_live_1234567890",            // Publishable key, not secret
	}

	for _, key := range invalidKeys {
		if re.MatchString(key) {
			t.Errorf("Pattern should NOT match invalid key: %s", key)
		}
	}
}

// TestGoogleAPIKeyPattern tests the Google API key pattern
func TestGoogleAPIKeyPattern(t *testing.T) {
	pattern := `AIza[0-9A-Za-z_-]{35}`
	re := regexp.MustCompile(pattern)

	// True positives - should match
	validKeys := []string{
		"AIzaSyDaGmWKa4JsXZ-HjGw7ISLn_3namBGewQe",
		"AIzaBCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	}

	for _, key := range validKeys {
		if !re.MatchString(key) {
			t.Errorf("Pattern should match valid Google API key: %s", key)
		}
	}

	// False positives - should NOT match
	invalidKeys := []string{
		"AIza123",                          // Too short
		"AIzaBCDEFGHIJ!@#$%^&*()",         // Invalid characters
		"NotAIza1234567890123456789012345", // Wrong prefix
	}

	for _, key := range invalidKeys {
		if re.MatchString(key) {
			t.Errorf("Pattern should NOT match invalid key: %s", key)
		}
	}
}
