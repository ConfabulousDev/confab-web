# email

Email sending via the Resend API, with per-user sliding-window rate limiting and a mock service for tests.

## Files

| File | Role |
|------|------|
| `email.go` | `Service` interface, `ResendService` implementation, `RateLimitedService` wrapper, `EmailRateLimiter`, HTML/text email templates, and `MockService` |
| `email_test.go` | Tests for `EmailRateLimiter` sliding window logic and `MockService` |
| `errors.go` | Package-level sentinel error `ErrRateLimitExceeded` |

## Key Types

- **`Service`** -- Interface with a single method `SendShareInvitation(ctx, ShareInvitationParams) error`.
- **`ResendService`** -- Production implementation that sends emails via the Resend HTTP API. Holds API key, from address/name, frontend URL, and an HTTP client with a 10-second timeout.
- **`RateLimitedService`** -- Wraps any `Service` with per-user hourly rate limiting. Checks the limit before delegating to the inner service.
- **`EmailRateLimiter`** -- Sliding-window rate limiter that tracks exact send timestamps per user ID. Thread-safe via `sync.Mutex`.
- **`ShareInvitationParams`** -- Parameters for a share invitation email: recipient, sharer info, session title, share URL, optional expiration.
- **`MockService`** -- Test double that records sent emails and can be configured to fail.

## Key API

- **`NewResendService(apiKey, fromAddress, fromName, frontendURL) *ResendService`** -- Creates a production email service.
- **`NewRateLimitedService(service Service, limitPerHour int) *RateLimitedService`** -- Wraps a service with rate limiting.
- **`(*RateLimitedService).SendShareInvitation(ctx, userID, params) error`** -- Checks rate limit, records the attempt, then sends. Returns `ErrRateLimitExceeded` if over limit.
- **`(*RateLimitedService).CheckRateLimit(userID, count) error`** -- Pre-checks whether `count` emails can be sent without actually sending.
- **`NewMockService() *MockService`** -- Creates a mock that records `SentEmails` for assertions.

## How to Extend

### Adding a new email type

1. Define a new params struct (like `ShareInvitationParams`).
2. Add a new method to the `Service` interface.
3. Implement the method on `ResendService` with HTML and text templates.
4. Add the method to `MockService` for testing.
5. If rate limiting applies, add a corresponding method on `RateLimitedService`.

## Invariants

- **Sliding window, not token bucket.** `EmailRateLimiter` tracks exact timestamps and counts emails within the last hour. This prevents bursts, unlike the token-bucket approach used in `internal/ratelimit`. The distinction is intentional (see code comment in `email.go`).
- **Thread safety.** `EmailRateLimiter` is protected by a `sync.Mutex`. All public methods acquire the lock.
- **Rate check before send.** `RateLimitedService.SendShareInvitation` checks the limit and records the attempt before calling the inner service. The count is incremented even if the send fails, preventing retries from bypassing the limit.
- **Both HTML and plain text.** Every email is sent with both an HTML body (using `html/template`) and a plain text fallback.

## Design Decisions

**Separate rate limiter from `internal/ratelimit`.** The generic rate limiter uses token buckets (`golang.org/x/time/rate`) which allow bursts. Email rate limiting requires strict "X per hour" enforcement to stay within provider quotas and prevent spam. A sliding-window algorithm with exact timestamp tracking achieves this.

**Mock service in production code.** `MockService` lives in the main package (not a `_test.go` file) so that other packages' tests can import and use it without circular dependencies.

**Resend as the email provider.** The `Service` interface abstracts the provider, so switching from Resend to another API only requires a new implementation.

## Testing

```bash
go test ./internal/email/...
```

Tests exercise the `EmailRateLimiter` sliding window logic and the `MockService`. The `ResendService` is not unit-tested against the real API; it relies on the interface abstraction and integration testing.

## Dependencies

**Uses:** `html/template` (email rendering)

**Used by:** `internal/api` (share invitation sending), `cmd/server/main.go` (service initialization)
