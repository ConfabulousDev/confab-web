package auth

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

func TestDeviceTokenIneligibleReason(t *testing.T) {
	active := func(email string) *models.User {
		return &models.User{Email: email, Status: models.UserStatusActive}
	}
	inactive := active("gone@example.com")
	inactive.Status = models.UserStatusInactive
	readOnly := active("demo@example.com")
	readOnly.ReadOnly = true

	for _, tc := range []struct {
		name           string
		user           *models.User
		allowedDomains []string
		want           string
	}{
		{"active user with no allow-list is eligible", active("a@example.com"), nil, ""},
		{"active user inside the allow-list is eligible", active("a@corp.test"), []string{"corp.test"}, ""},
		{"deleted user is ineligible", nil, nil, "user_not_found"},
		{"inactive user is ineligible", inactive, nil, "user_inactive"},
		{"read-only user is ineligible", readOnly, nil, "user_read_only"},
		{"user outside the allow-list is ineligible", active("a@example.com"), []string{"corp.test"}, "email_domain_not_permitted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := deviceTokenIneligibleReason(tc.user, tc.allowedDomains); got != tc.want {
				t.Errorf("deviceTokenIneligibleReason = %q, want %q", got, tc.want)
			}
		})
	}
}
