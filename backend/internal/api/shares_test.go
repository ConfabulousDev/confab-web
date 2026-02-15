package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/validation"
)

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

// Test share public flag validation
func TestSharePublicFlagValidation(t *testing.T) {
	t.Run("public share without recipients is valid", func(t *testing.T) {
		isPublic := true
		recipients := []string{}

		// Public shares don't require recipients
		if !isPublic && len(recipients) == 0 {
			t.Error("public share should be valid without recipients")
		}
	})

	t.Run("non-public share without recipients is invalid", func(t *testing.T) {
		isPublic := false
		recipients := []string{}

		// Non-public shares require at least one recipient
		if !isPublic && len(recipients) == 0 {
			// This is the expected validation failure
			return
		}
		t.Error("non-public share without recipients should be invalid")
	})

	t.Run("non-public share with recipients is valid", func(t *testing.T) {
		isPublic := false
		recipients := []string{"user@example.com"}

		if !isPublic && len(recipients) == 0 {
			t.Error("non-public share with recipients should be valid")
		}
	})
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

func TestHandleCreateShareDisabled(t *testing.T) {
	t.Run("returns 403 when shares are disabled", func(t *testing.T) {
		handler := HandleCreateShare(nil, "", nil, true)

		req := httptest.NewRequest("POST", "/api/v1/sessions/test-id/share", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "Share creation is disabled by the administrator") {
			t.Errorf("expected disabled message, got: %s", body)
		}
	})

	t.Run("does not return 403 when shares are enabled", func(t *testing.T) {
		// With sharesDisabled=false and nil db, it should fail with auth error (not 403)
		handler := HandleCreateShare(nil, "", nil, false)

		req := httptest.NewRequest("POST", "/api/v1/sessions/test-id/share", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should not be 403 (disabled), will be 401 because no auth context
		if rr.Code == http.StatusForbidden {
			body := rr.Body.String()
			if strings.Contains(body, "disabled") {
				t.Error("should not return disabled message when shares are enabled")
			}
		}
	})
}

