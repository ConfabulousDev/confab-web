package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

// TestGeneratePKCE verifies the RFC 7636 S256 contract (r9zn): the challenge is
// base64url-no-pad(SHA256(verifier)), both stay in the unreserved charset, and
// two calls produce distinct verifiers.
func TestGeneratePKCE(t *testing.T) {
	v1, c1, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE: %v", err)
	}
	if v1 == "" || c1 == "" {
		t.Fatal("verifier/challenge must be non-empty")
	}
	// Challenge is the S256 of the verifier.
	sum := sha256.Sum256([]byte(v1))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); want != c1 {
		t.Errorf("challenge = %q, want S256 %q", c1, want)
	}
	// RFC 7636 verifier length: 43 chars for 32 raw bytes, base64url no padding.
	if len(v1) != 43 {
		t.Errorf("verifier length = %d, want 43", len(v1))
	}
	// Charset: base64url unreserved only (no '+', '/', or '=').
	for _, ch := range v1 + c1 {
		ok := (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_'
		if !ok {
			t.Errorf("non-unreserved char %q in PKCE value", ch)
		}
	}
	// Uniqueness across calls.
	v2, _, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE (2nd): %v", err)
	}
	if v1 == v2 {
		t.Error("two generatePKCE calls produced identical verifiers")
	}
}

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
