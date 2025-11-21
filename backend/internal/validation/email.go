package validation

import (
	"regexp"
	"strings"
)

// emailRegex validates common email formats
// Requires: local-part @ domain with at least one dot
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)

// IsValidEmail checks if an email address is valid
// Returns true if the email matches expected format with proper domain (requires TLD)
func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)

	// Basic length checks
	if len(email) == 0 || len(email) > 254 {
		return false
	}

	// Check format with regex (requires domain with TLD)
	if !emailRegex.MatchString(email) {
		return false
	}

	// Additional check: no consecutive dots in local part
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if strings.Contains(parts[0], "..") {
		return false
	}

	return true
}
