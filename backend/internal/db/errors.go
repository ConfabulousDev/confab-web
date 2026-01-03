package db

import "errors"

// Sentinel errors for type-safe error checking
// Use errors.Is() instead of string comparison
var (
	// Session errors
	ErrSessionNotFound = errors.New("session not found")
	ErrUnauthorized    = errors.New("unauthorized")

	// Share errors
	ErrShareNotFound = errors.New("share not found")
	ErrShareExpired  = errors.New("share expired")
	ErrForbidden     = errors.New("forbidden")

	// File errors
	ErrFileNotFound = errors.New("file not found")

	// User errors
	ErrUserNotFound   = errors.New("user not found")
	ErrOwnerInactive  = errors.New("session owner is inactive")

	// API key errors
	ErrAPIKeyNotFound      = errors.New("API key not found")
	ErrAPIKeyLimitExceeded = errors.New("API key limit exceeded")
	ErrAPIKeyNameExists    = errors.New("API key with this name already exists")

	// Rate limiting errors
	ErrRateLimitExceeded = errors.New("weekly upload limit exceeded")

	// Device code errors
	ErrDeviceCodeNotFound = errors.New("device code not found or expired")

	// GitHub link errors
	ErrGitHubLinkNotFound  = errors.New("github link not found")
	ErrGitHubLinkDuplicate = errors.New("github link already exists")
)
