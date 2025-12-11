package admin

import (
	"os"
	"strings"
)

// IsSuperAdmin checks if the given email is in the super admin list
func IsSuperAdmin(email string) bool {
	adminEmails := os.Getenv("SUPER_ADMIN_EMAILS")
	if adminEmails == "" {
		return false
	}

	email = strings.ToLower(strings.TrimSpace(email))
	for _, adminEmail := range strings.Split(adminEmails, ",") {
		if strings.ToLower(strings.TrimSpace(adminEmail)) == email {
			return true
		}
	}
	return false
}
