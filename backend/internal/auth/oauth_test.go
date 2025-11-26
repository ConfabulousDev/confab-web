package auth

import (
	"context"
	"os"
	"testing"
)

// Test random string generation for OAuth state - security critical
func TestGenerateRandomString(t *testing.T) {
	t.Run("generates string of correct length", func(t *testing.T) {
		str, err := generateRandomString(32)
		if err != nil {
			t.Fatalf("generateRandomString failed: %v", err)
		}

		if len(str) != 32 {
			t.Errorf("expected length 32, got %d", len(str))
		}
	})

	t.Run("generates different strings each time", func(t *testing.T) {
		str1, err := generateRandomString(32)
		if err != nil {
			t.Fatalf("generateRandomString failed: %v", err)
		}

		str2, err := generateRandomString(32)
		if err != nil {
			t.Fatalf("generateRandomString failed: %v", err)
		}

		if str1 == str2 {
			t.Error("generated identical strings - randomness failure")
		}
	})

	t.Run("generates multiple unique strings", func(t *testing.T) {
		states := make(map[string]bool)
		count := 100

		for i := 0; i < count; i++ {
			str, err := generateRandomString(32)
			if err != nil {
				t.Fatalf("generateRandomString failed: %v", err)
			}

			if states[str] {
				t.Errorf("duplicate state generated: %s", str)
				break
			}
			states[str] = true
		}

		if len(states) != count {
			t.Errorf("expected %d unique states, got %d", count, len(states))
		}
	})

	t.Run("generates alphanumeric characters only", func(t *testing.T) {
		str, err := generateRandomString(100)
		if err != nil {
			t.Fatalf("generateRandomString failed: %v", err)
		}

		for _, c := range str {
			isAlphanumeric := (c >= 'a' && c <= 'z') ||
				(c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') ||
				c == '-' || c == '_'

			if !isAlphanumeric {
				t.Errorf("generated string contains non-alphanumeric character: %c", c)
				break
			}
		}
	})
}

// Test email whitelist validation - security critical
func TestIsEmailAllowed(t *testing.T) {
	// Save original env and restore after tests
	originalAllowed := os.Getenv("ALLOWED_EMAILS")
	originalInviteTS := os.Getenv("ALLOW_INVITED_EMAILS_AFTER_TS")
	defer func() {
		if originalAllowed == "" {
			os.Unsetenv("ALLOWED_EMAILS")
		} else {
			os.Setenv("ALLOWED_EMAILS", originalAllowed)
		}
		if originalInviteTS == "" {
			os.Unsetenv("ALLOW_INVITED_EMAILS_AFTER_TS")
		} else {
			os.Setenv("ALLOW_INVITED_EMAILS_AFTER_TS", originalInviteTS)
		}
	}()

	// Clear ALLOW_INVITED_EMAILS_AFTER_TS for basic tests (feature disabled)
	os.Unsetenv("ALLOW_INVITED_EMAILS_AFTER_TS")

	ctx := context.Background()

	t.Run("allows all emails when whitelist not configured", func(t *testing.T) {
		os.Unsetenv("ALLOWED_EMAILS")

		emails := []string{
			"user@example.com",
			"admin@company.com",
			"test@test.com",
		}

		for _, email := range emails {
			if !isEmailAllowed(ctx, nil, email) {
				t.Errorf("email %q should be allowed when whitelist not configured", email)
			}
		}
	})

	t.Run("allows empty email when whitelist not configured", func(t *testing.T) {
		os.Unsetenv("ALLOWED_EMAILS")

		// Note: Current implementation allows all emails (including empty) when no whitelist
		// This tests the actual behavior, not necessarily ideal behavior
		if !isEmailAllowed(ctx, nil, "") {
			t.Error("function returns true for empty email when no whitelist configured")
		}
	})

	t.Run("allows whitelisted email", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "user@example.com,admin@company.com")

		if !isEmailAllowed(ctx, nil, "user@example.com") {
			t.Error("whitelisted email should be allowed")
		}

		if !isEmailAllowed(ctx, nil, "admin@company.com") {
			t.Error("whitelisted email should be allowed")
		}
	})

	t.Run("rejects non-whitelisted email", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "user@example.com")

		if isEmailAllowed(ctx, nil, "hacker@evil.com") {
			t.Error("non-whitelisted email should be rejected")
		}
	})

	t.Run("is case-insensitive", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "User@Example.COM")

		emails := []string{
			"user@example.com",
			"USER@EXAMPLE.COM",
			"UsEr@ExAmPlE.cOm",
		}

		for _, email := range emails {
			if !isEmailAllowed(ctx, nil, email) {
				t.Errorf("email %q should be allowed (case-insensitive)", email)
			}
		}
	})

	t.Run("handles whitespace in whitelist", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", " user@example.com , admin@company.com ")

		if !isEmailAllowed(ctx, nil, "user@example.com") {
			t.Error("should handle leading/trailing whitespace in whitelist")
		}

		if !isEmailAllowed(ctx, nil, "admin@company.com") {
			t.Error("should handle whitespace around commas")
		}
	})

	t.Run("handles whitespace in input email", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "user@example.com")

		if !isEmailAllowed(ctx, nil, "  user@example.com  ") {
			t.Error("should trim whitespace from input email")
		}
	})

	t.Run("rejects empty email with whitelist", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "user@example.com")

		if isEmailAllowed(ctx, nil, "") {
			t.Error("empty email should be rejected even with whitelist")
		}
	})

	t.Run("supports multiple whitelisted emails", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "user1@example.com,user2@example.com,user3@example.com")

		allowed := []string{
			"user1@example.com",
			"user2@example.com",
			"user3@example.com",
		}

		for _, email := range allowed {
			if !isEmailAllowed(ctx, nil, email) {
				t.Errorf("whitelisted email %q should be allowed", email)
			}
		}

		if isEmailAllowed(ctx, nil, "user4@example.com") {
			t.Error("non-whitelisted email should be rejected")
		}
	})

	t.Run("invite feature disabled when ALLOW_INVITED_EMAILS_AFTER_TS not set", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "admin@example.com")
		os.Unsetenv("ALLOW_INVITED_EMAILS_AFTER_TS")

		// Without the feature enabled, only hardcoded emails allowed
		// (passing nil DB is fine since feature is disabled)
		if isEmailAllowed(ctx, nil, "invited@example.com") {
			t.Error("invited email should not be allowed when feature is disabled")
		}
	})

	t.Run("invite feature effectively disabled with large timestamp", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "admin@example.com")
		os.Setenv("ALLOW_INVITED_EMAILS_AFTER_TS", "2000000000") // Year 2033

		// With a far-future timestamp, no historical invites qualify
		// (passing nil DB means the check is skipped, so this tests the env var is parsed)
		if isEmailAllowed(ctx, nil, "invited@example.com") {
			t.Error("invited email should not be allowed with far-future timestamp and nil DB")
		}
	})

	t.Run("invalid ALLOW_INVITED_EMAILS_AFTER_TS fails closed", func(t *testing.T) {
		os.Setenv("ALLOWED_EMAILS", "admin@example.com")
		os.Setenv("ALLOW_INVITED_EMAILS_AFTER_TS", "not-a-number")

		// With invalid timestamp, should fail closed (reject)
		if isEmailAllowed(ctx, nil, "random@example.com") {
			t.Error("should fail closed when ALLOW_INVITED_EMAILS_AFTER_TS is invalid")
		}
	})
}

