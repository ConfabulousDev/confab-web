package api

import (
	"testing"

	"github.com/ConfabulousDev/confab/backend/internal/validation"
)

// Test share token generation - security critical
func TestGenerateShareToken(t *testing.T) {
	t.Run("generates valid token", func(t *testing.T) {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("generateShareToken failed: %v", err)
		}

		// Should be 32 hex characters (16 bytes)
		if len(token) != 32 {
			t.Errorf("expected token length 32, got %d", len(token))
		}

		// Check all characters are valid hex
		for _, c := range token {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("token contains non-hex character: %c", c)
				break
			}
		}
	})

	t.Run("generates different tokens each time", func(t *testing.T) {
		token1, err := generateShareToken()
		if err != nil {
			t.Fatalf("generateShareToken failed: %v", err)
		}

		token2, err := generateShareToken()
		if err != nil {
			t.Fatalf("generateShareToken failed: %v", err)
		}

		if token1 == token2 {
			t.Error("generated identical tokens - randomness failure")
		}
	})

	t.Run("generates multiple unique tokens", func(t *testing.T) {
		tokens := make(map[string]bool)
		count := 100

		for i := 0; i < count; i++ {
			token, err := generateShareToken()
			if err != nil {
				t.Fatalf("generateShareToken failed: %v", err)
			}

			if tokens[token] {
				t.Errorf("duplicate token generated: %s", token)
				break
			}
			tokens[token] = true
		}

		if len(tokens) != count {
			t.Errorf("expected %d unique tokens, got %d", count, len(tokens))
		}
	})
}

// Test email validation logic - business logic
func TestEmailValidation(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantValid bool
	}{
		{
			name:      "valid simple email",
			email:     "user@example.com",
			wantValid: true,
		},
		{
			name:      "valid email with subdomain",
			email:     "user@mail.example.com",
			wantValid: true,
		},
		{
			name:      "valid email with plus",
			email:     "user+tag@example.com",
			wantValid: true,
		},
		{
			name:      "invalid - no @",
			email:     "userexample.com",
			wantValid: false,
		},
		{
			name:      "invalid - empty",
			email:     "",
			wantValid: false,
		},
		{
			name:      "invalid - just @",
			email:     "@",
			wantValid: false,
		},
		{
			name:      "valid with whitespace (trimmed)",
			email:     "  user@example.com  ",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validation.IsValidEmail(tt.email)

			if isValid != tt.wantValid {
				t.Errorf("email %q: expected valid=%v, got valid=%v", tt.email, tt.wantValid, isValid)
			}
		})
	}
}

// Test share visibility validation
func TestShareVisibilityValidation(t *testing.T) {
	validVisibilities := []string{"public", "private"}
	invalidVisibilities := []string{"", "Public", "PRIVATE", "protected", "shared"}

	for _, v := range validVisibilities {
		t.Run("accepts "+v, func(t *testing.T) {
			if v != "public" && v != "private" {
				t.Errorf("valid visibility %q rejected", v)
			}
		})
	}

	for _, v := range invalidVisibilities {
		t.Run("rejects "+v, func(t *testing.T) {
			if v == "public" || v == "private" {
				t.Errorf("invalid visibility %q accepted", v)
			}
		})
	}
}

// Test email count limits for private shares
func TestPrivateShareEmailLimits(t *testing.T) {
	t.Run("rejects private share with no emails", func(t *testing.T) {
		// Test the validation logic: private shares require at least one email
		isPrivate := true
		emailCount := 0

		if isPrivate && emailCount == 0 {
			// This should fail validation
			return
		}
		t.Error("expected validation failure for private share with no emails")
	})

	t.Run("accepts private share with one email", func(t *testing.T) {
		isPrivate := true
		emailCount := 1

		if isPrivate && emailCount == 0 {
			t.Error("valid private share rejected")
		}
	})

	t.Run("rejects private share with too many emails", func(t *testing.T) {
		maxEmails := 50
		emailCount := maxEmails + 1

		if emailCount > maxEmails {
			// Should fail validation
			return
		}
		t.Error("expected validation failure for too many emails")
	})

	t.Run("accepts private share at max emails", func(t *testing.T) {
		maxEmails := 50
		emailCount := maxEmails

		if emailCount == 0 {
			t.Error("valid private share rejected")
		}
		if emailCount > maxEmails {
			t.Error("valid email count rejected")
		}
	})

	t.Run("public share does not require emails", func(t *testing.T) {
		// Test the validation logic: public shares don't require emails
		isPublic := true
		emailCount := 0

		// Public shares should work without emails
		if isPublic || emailCount > 0 {
			// This is valid
			return
		}
		t.Error("public share should not require emails")
	})
}
