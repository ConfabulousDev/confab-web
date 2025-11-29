package utils

import "time"

// TruncateSecret safely truncates a secret string for display.
// Returns a string like "abc123...wxyz" showing prefix and suffix.
// If the string is too short, returns a masked version.
func TruncateSecret(s string, prefixLen, suffixLen int) string {
	minLen := prefixLen + suffixLen
	if len(s) < minLen {
		// String too short - mask it entirely
		if len(s) == 0 {
			return "(empty)"
		}
		return "***"
	}
	return s[:prefixLen] + "..." + s[len(s)-suffixLen:]
}

// TruncateWithEllipsis shortens a string for display by keeping the end
// and adding ellipsis at the beginning if it exceeds maxLen
func TruncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen+3:]
}

// TruncateEnd shortens a string for display by keeping the beginning
// and adding ellipsis at the end if it exceeds maxLen
func TruncateEnd(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// HTTP client timeouts
const (
	// DefaultHTTPTimeout is used for quick API calls like validation
	DefaultHTTPTimeout = 30 * time.Second

	// UploadHTTPTimeout is used for potentially large file uploads
	UploadHTTPTimeout = 5 * time.Minute
)

// HTTP server timeouts
const (
	// ServerReadTimeout is the maximum duration for reading the entire request
	ServerReadTimeout = 30 * time.Second

	// ServerWriteTimeout is the maximum duration before timing out writes of the response
	ServerWriteTimeout = 30 * time.Second

	// ServerIdleTimeout is the maximum duration to wait for the next request when keep-alives are enabled
	ServerIdleTimeout = 60 * time.Second
)

// User interaction timeouts
const (
	// OAuthFlowTimeout is the maximum time to wait for user to complete OAuth authentication
	OAuthFlowTimeout = 5 * time.Minute
)
