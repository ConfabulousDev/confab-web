package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// emailRegex validates common email formats
// Requires: local-part @ domain with at least one dot
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)

// domainRegex validates domain format: labels separated by dots, each label starts/ends with alnum
var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)

// IsAllowedEmailDomain checks if an email's domain is in the allowed list.
// Returns true if allowedDomains is empty (no restriction).
// Performs strict, case-insensitive domain match (no subdomain matching).
func IsAllowedEmailDomain(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}

	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])

	for _, allowed := range allowedDomains {
		if domain == allowed {
			return true
		}
	}
	return false
}

// ValidateDomainList validates a list of domain entries for correctness.
// Each domain must have a valid format with a TLD (e.g., "company.com").
// Returns an error describing the first invalid entry.
func ValidateDomainList(domains []string) error {
	for _, d := range domains {
		if d == "" {
			return fmt.Errorf("empty domain entry")
		}
		if strings.ContainsAny(d, " \t\n\r") {
			return fmt.Errorf("domain %q contains whitespace", d)
		}
		if !domainRegex.MatchString(d) {
			return fmt.Errorf("domain %q is not a valid domain (must have TLD, e.g. company.com)", d)
		}
	}
	return nil
}

// NormalizeEmail lowercases and trims whitespace from an email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

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
