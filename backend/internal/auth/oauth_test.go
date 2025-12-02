package auth

import (
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

// Note: canUserLogin tests are in auth_integration_test.go since they require a database

