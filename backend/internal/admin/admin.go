package admin

import (
	"os"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// IsSuperAdmin checks if the given email is in the super admin list
func IsSuperAdmin(email string) bool {
	adminEmails := os.Getenv("SUPER_ADMIN_EMAILS")
	if adminEmails == "" {
		return false
	}

	email = validation.NormalizeEmail(email)
	for _, adminEmail := range strings.Split(adminEmails, ",") {
		if validation.NormalizeEmail(adminEmail) == email {
			return true
		}
	}
	return false
}
