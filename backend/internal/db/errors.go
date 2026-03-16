package db

import "errors"

// Sentinel errors for type-safe error checking
// Use errors.Is() instead of string comparison
var (
	// Session errors
	ErrSessionNotFound = errors.New("session not found")
	ErrUnauthorized    = errors.New("unauthorized")

	// Share errors
	ErrForbidden = errors.New("forbidden")

	// File errors
	ErrFileNotFound = errors.New("file not found")

	// User errors
	ErrUserNotFound   = errors.New("user not found")
	ErrOwnerInactive  = errors.New("session owner is inactive")

	// API key errors
	ErrAPIKeyNotFound      = errors.New("API key not found")
	ErrAPIKeyLimitExceeded = errors.New("API key limit exceeded")
	ErrAPIKeyNameExists    = errors.New("API key with this name already exists")

	// Device code errors
	ErrDeviceCodeNotFound = errors.New("device code not found or expired")

	// GitHub link errors
	ErrGitHubLinkNotFound = errors.New("github link not found")

	// Password authentication errors
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("account is temporarily locked")

	// TIL errors
	ErrTILNotFound = errors.New("TIL not found")
)
