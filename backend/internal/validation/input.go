package validation

import (
	"fmt"
	"unicode/utf8"
)

// Validation limits for URL parameters
const (
	MinExternalIDLength = 1 // Min external ID length
)

// Field size limits (must match DB VARCHAR constraints in migration 000010, 000011)
const (
	MaxAvatarURLLength        = 4096 // users.avatar_url
	MaxExternalIDLength       = 512  // sessions.external_id
	MaxSummaryLength          = 2048 // sessions.summary
	MaxFirstUserMessageLength = 8192 // sessions.first_user_message
	MaxCWDLength              = 8192 // sessions.cwd, runs.cwd
	MaxTranscriptPathLength   = 8192 // sessions.transcript_path, runs.transcript_path
	MaxRunReasonLength        = 2048 // runs.reason
	MaxFilePathLength         = 8192 // files.file_path
	MaxS3KeyLength            = 2048 // files.s3_key
	MaxSyncFileNameLength     = 512  // sync_files.file_name
	MaxHostnameLength         = 255  // sessions.hostname
	MaxUsernameLength         = 255  // sessions.username
)

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

// ValidateCWD validates a working directory path
func ValidateCWD(cwd string) error {
	if len(cwd) > MaxCWDLength {
		return fmt.Errorf("cwd exceeds maximum length of %d characters", MaxCWDLength)
	}
	return nil
}

// ValidateTranscriptPath validates a transcript file path
func ValidateTranscriptPath(path string) error {
	if len(path) > MaxTranscriptPathLength {
		return fmt.Errorf("transcript_path exceeds maximum length of %d characters", MaxTranscriptPathLength)
	}
	return nil
}

// ValidateSyncFileName validates a sync file name
func ValidateSyncFileName(fileName string) error {
	if len(fileName) > MaxSyncFileNameLength {
		return fmt.Errorf("file_name exceeds maximum length of %d characters", MaxSyncFileNameLength)
	}
	return nil
}

// ValidateSummary validates a session summary
func ValidateSummary(summary string) error {
	if len(summary) > MaxSummaryLength {
		return fmt.Errorf("summary exceeds maximum length of %d characters", MaxSummaryLength)
	}
	return nil
}

// ValidateFirstUserMessage validates a first user message
func ValidateFirstUserMessage(msg string) error {
	if len(msg) > MaxFirstUserMessageLength {
		return fmt.Errorf("first_user_message exceeds maximum length of %d characters", MaxFirstUserMessageLength)
	}
	return nil
}

// MaxAPIKeyNameLength is the maximum length for API key names
const MaxAPIKeyNameLength = 255

// ValidateAPIKeyName validates an API key name
func ValidateAPIKeyName(name string) error {
	if len(name) > MaxAPIKeyNameLength {
		return fmt.Errorf("key name exceeds maximum length of %d characters", MaxAPIKeyNameLength)
	}
	return nil
}

// ValidateHostname validates a client hostname
func ValidateHostname(hostname string) error {
	if len(hostname) > MaxHostnameLength {
		return fmt.Errorf("hostname exceeds maximum length of %d characters", MaxHostnameLength)
	}
	return nil
}

// ValidateUsername validates a client username
func ValidateUsername(username string) error {
	if len(username) > MaxUsernameLength {
		return fmt.Errorf("username exceeds maximum length of %d characters", MaxUsernameLength)
	}
	return nil
}

// TODO(2026-Q2): Remove truncation helpers when grace period ends

// TruncateSyncFileName truncates a sync file name to the maximum allowed length
func TruncateSyncFileName(s string) string {
	return truncateString(s, MaxSyncFileNameLength)
}

// TruncateSummary truncates a summary to the maximum allowed length
func TruncateSummary(s string) string {
	return truncateString(s, MaxSummaryLength)
}

// TruncateFirstUserMessage truncates a first user message to the maximum allowed length
func TruncateFirstUserMessage(s string) string {
	return truncateString(s, MaxFirstUserMessageLength)
}

// truncateString truncates a string to maxLen bytes, ensuring valid UTF-8
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Truncate to maxLen bytes, but ensure we don't cut a UTF-8 character in half
	truncated := s[:maxLen]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}
