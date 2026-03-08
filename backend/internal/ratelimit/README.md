# ratelimit

In-memory token-bucket rate limiter with HTTP middleware for per-IP and per-user request throttling.

## Files

| File | Role |
|------|------|
| `ratelimit.go` | `RateLimiter` interface and `InMemoryRateLimiter` implementation with background cleanup |
| `middleware.go` | HTTP middleware and handler wrappers that enforce rate limits |
| `middleware_test.go` | Tests for middleware, `MiddlewareWithKey`, and `HandlerFunc` |

## Key Types

- **`RateLimiter`** -- Interface with `Allow(ctx, key) bool` and `AllowN(ctx, key, n) bool`. Allows swapping between in-memory and distributed (e.g., Redis) implementations.
- **`InMemoryRateLimiter`** -- Token-bucket implementation using `golang.org/x/time/rate`. One bucket per key (IP, user ID, etc.), with background goroutine cleanup of stale buckets.

## Key API

### Rate limiter

- **`NewInMemoryRateLimiter(rps float64, burst int) *InMemoryRateLimiter`** -- Creates a limiter. `rps` is requests per second; `burst` is the maximum burst size. Starts a background cleanup goroutine.
- **`(*InMemoryRateLimiter).Allow(ctx, key) bool`** -- Checks if one request is allowed for the given key.
- **`(*InMemoryRateLimiter).AllowN(ctx, key, n) bool`** -- Checks if `n` requests are allowed.
- **`(*InMemoryRateLimiter).Stop()`** -- Stops the background cleanup goroutine.
- **`(*InMemoryRateLimiter).Stats() map[string]interface{}`** -- Returns diagnostic info: active limiter count, rate, burst, and type.

### Middleware

- **`Middleware(limiter RateLimiter) func(http.Handler) http.Handler`** -- Rate limits by composite client IP key from `clientip.FromRequest`.
- **`MiddlewareWithKey(limiter, keyFunc) func(http.Handler) http.Handler`** -- Rate limits using a custom key extractor. Falls back to IP key if the function returns empty.
- **`HandlerFunc(limiter, handler) http.HandlerFunc`** -- Wraps a single handler with rate limiting. Useful for per-endpoint limits.
- **`UserKeyFunc(userIDKey) func(*http.Request) string`** -- Key extractor that reads a user ID from context for per-user rate limiting.

## How to Extend

### Adding a distributed (Redis) rate limiter

1. Create a new type that implements the `RateLimiter` interface.
2. Implement `Allow` and `AllowN` using Redis commands (e.g., `INCR` with `EXPIRE`, or a Lua script for sliding windows).
3. No changes to middleware are needed; it works with any `RateLimiter` implementation.

### Adding per-endpoint rate limits

1. Create a separate `InMemoryRateLimiter` with the desired rate and burst.
2. Wrap the endpoint handler with `HandlerFunc(limiter, handler)` or apply `Middleware(limiter)` to a sub-router.

## Invariants

- **Middleware depends on `clientip.Middleware`.** The default `Middleware` reads `clientip.FromRequest(r).RateLimitKey` from context. If `clientip.Middleware` has not run, the key will be empty and all requests will share a single bucket.
- **Background cleanup prevents memory leaks.** Stale buckets (unused for 10 minutes) are cleaned up every 5 minutes by a background goroutine.
- **`Stop()` must be called on shutdown.** Failing to call `Stop` will leak the cleanup goroutine.
- **Concurrent safety.** `sync.Map` is used for the limiter and last-access stores; no external locking is needed.

## Design Decisions

**Token bucket via `golang.org/x/time/rate`.** Token buckets allow controlled bursts (up to `burst` size) while maintaining a steady-state rate. This is appropriate for API rate limiting where short bursts are acceptable.

**Interface for swappability.** The `RateLimiter` interface allows replacing the in-memory implementation with a distributed one (Redis, etc.) for multi-instance deployments without changing middleware code.

**Composite IP key from `clientip`.** Using the composite key (all observed IPs joined with `|`) instead of a single header makes rate limiting resistant to IP spoofing via proxy headers.

## Testing

```bash
go test ./internal/ratelimit/...
```

Tests cover middleware behavior (allowed and blocked requests), the `MiddlewareWithKey` custom key path, and the `HandlerFunc` wrapper.

## Dependencies

**Uses:** `internal/clientip`, `internal/logger`, `golang.org/x/time/rate`

**Used by:** `internal/api` (server middleware chain)
