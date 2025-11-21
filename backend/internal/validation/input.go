package validation

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Validation limits for URL parameters
const (
	MaxSessionIDLength = 256 // Max session ID length
	MinSessionIDLength = 1   // Min session ID length
	ShareTokenLength   = 32  // Share tokens are exactly 32 hex chars
)

// hexRegex matches hexadecimal strings
var hexRegex = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// ValidateSessionID validates a session ID from URL parameters
// Returns error if session ID is invalid
func ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if len(sessionID) < MinSessionIDLength || len(sessionID) > MaxSessionIDLength {
		return fmt.Errorf("session_id must be between %d and %d characters", MinSessionIDLength, MaxSessionIDLength)
	}
	if !utf8.ValidString(sessionID) {
		return fmt.Errorf("session_id must be valid UTF-8")
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
