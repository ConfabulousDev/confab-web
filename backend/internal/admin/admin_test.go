package admin_test

import (
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestIsSuperAdmin(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		email      string
		wantResult bool
	}{
		{
			name:       "empty env returns false",
			envValue:   "",
			email:      "test@example.com",
			wantResult: false,
		},
		{
			name:       "single admin match",
			envValue:   "admin@example.com",
			email:      "admin@example.com",
			wantResult: true,
		},
		{
			name:       "single admin no match",
			envValue:   "admin@example.com",
			email:      "user@example.com",
			wantResult: false,
		},
		{
			name:       "multiple admins first match",
			envValue:   "admin1@example.com,admin2@example.com",
			email:      "admin1@example.com",
			wantResult: true,
		},
		{
			name:       "multiple admins second match",
			envValue:   "admin1@example.com,admin2@example.com",
			email:      "admin2@example.com",
			wantResult: true,
		},
		{
			name:       "multiple admins no match",
			envValue:   "admin1@example.com,admin2@example.com",
			email:      "user@example.com",
			wantResult: false,
		},
		{
			name:       "case insensitive match",
			envValue:   "Admin@Example.com",
			email:      "admin@example.com",
			wantResult: true,
		},
		{
			name:       "whitespace handling",
			envValue:   "  admin@example.com  ,  admin2@example.com  ",
			email:      "admin@example.com",
			wantResult: true,
		},
		{
			name:       "input email with whitespace",
			envValue:   "admin@example.com",
			email:      "  admin@example.com  ",
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			oldValue := os.Getenv("SUPER_ADMIN_EMAILS")
			os.Setenv("SUPER_ADMIN_EMAILS", tt.envValue)
			defer os.Setenv("SUPER_ADMIN_EMAILS", oldValue)

			got := admin.IsSuperAdmin(tt.email)
			if got != tt.wantResult {
				t.Errorf("IsSuperAdmin(%q) = %v, want %v", tt.email, got, tt.wantResult)
			}
		})
	}
}

// TestIsEffectiveAdmin pins the admin union (env SUPER_ADMIN_EMAILS ∪
// users.is_admin) shared by Middleware, /me and session access (xtrf).
func TestIsEffectiveAdmin(t *testing.T) {
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "env@example.com")

	tests := []struct {
		name string
		user *models.User
		want bool
	}{
		{"nil user is not admin", nil, false},
		{"env super-admin", &models.User{Email: "env@example.com"}, true},
		{"env super-admin, case-insensitive", &models.User{Email: "ENV@example.com"}, true},
		{"is_admin column", &models.User{Email: "db@example.com", IsAdmin: true}, true},
		{"neither", &models.User{Email: "plain@example.com"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := admin.IsEffectiveAdmin(tt.user); got != tt.want {
				t.Errorf("IsEffectiveAdmin(%v) = %v, want %v", tt.user, got, tt.want)
			}
		})
	}
}
