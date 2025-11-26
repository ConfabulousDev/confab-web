package email

import "errors"

var (
	// ErrRateLimitExceeded is returned when the email rate limit is exceeded
	ErrRateLimitExceeded = errors.New("email rate limit exceeded")
)
