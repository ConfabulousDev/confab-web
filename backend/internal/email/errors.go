package email

import "errors"

// ErrRateLimitExceeded is returned when the email rate limit is exceeded
var ErrRateLimitExceeded = errors.New("email rate limit exceeded")
