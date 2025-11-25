package validation

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Validation limits for URL parameters
const (
	MaxExternalIDLength = 256 // Max external ID length
	MinExternalIDLength = 1   // Min external ID length
	ShareTokenLength    = 32  // Share tokens are exactly 32 hex chars
)

// hexRegex matches hexadecimal strings
var hexRegex = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// ValidateExternalID validates an external ID from URL parameters
// Returns error if external ID is invalid
func ValidateExternalID(externalID string) error {
	if externalID == "" {
		return fmt.Errorf("external_id is required")
	}
	if len(externalID) < MinExternalIDLength || len(externalID) > MaxExternalIDLength {
		return fmt.Errorf("external_id must be between %d and %d characters", MinExternalIDLength, MaxExternalIDLength)
	}
	if !utf8.ValidString(externalID) {
		return fmt.Errorf("external_id must be valid UTF-8")
	}
	return nil
}

// ValidateShareToken validates a share token from URL parameters
// Share tokens must be exactly 32 hexadecimal characters
func ValidateShareToken(shareToken string) error {
	if shareToken == "" {
		return fmt.Errorf("share_token is required")
	}
	if len(shareToken) != ShareTokenLength {
		return fmt.Errorf("share_token must be exactly %d characters", ShareTokenLength)
	}
	if !hexRegex.MatchString(shareToken) {
		return fmt.Errorf("share_token must be hexadecimal")
	}
	return nil
}
