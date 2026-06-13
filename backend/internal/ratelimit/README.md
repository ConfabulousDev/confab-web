# ratelimit

In-memory token-bucket rate limiter with HTTP middleware for per-IP and per-user request throttling.

## Files

| File | Role |
|------|------|
| `ratelimit.go` | `RateLimiter` interface and `InMemoryRateLimiter` implementation with background cleanup and a bucket-count cap |
| `middleware.go` | HTTP middleware and handler wrappers that enforce rate limits |
| `middleware_test.go` | Tests for `clientip` integration and rate-limit key derivation |
| `ratelimit_test.go` | Tests for the token-bucket implementation (`Allow`, `AllowN`, burst, per-key isolation, concurrent `getLimiter`, `cleanupOldLimiters`, bucket-cap eviction, `Stop`) plus middleware behavior (`Middleware`, `MiddlewareWithKey`, `HandlerFunc`, `UserKeyFunc`) |

## Key Types

- **`RateLimiter`** -- Interface with `Allow(ctx, key) bool` and `AllowN(ctx, key, n) bool`. Allows swapping between in-memory and distributed (e.g., Redis) implementations.
- **`InMemoryRateLimiter`** -- Token-bucket implementation using `golang.org/x/time/rate`. One bucket per key (IP, user ID, etc.), with background goroutine cleanup of stale buckets.

## Key API

### Rate limiter

- **`NewInMemoryRateLimiter(rps float64, burst int, maxBuckets int) *InMemoryRateLimiter`** -- Creates a limiter. `rps` is requests per second; `burst` is the maximum burst size; `maxBuckets` caps the number of live per-key buckets (a value `<= 0` disables the cap). Starts a background cleanup goroutine.
- **`(*InMemoryRateLimiter).Allow(ctx, key) bool`** -- Checks if one request is allowed for the given key.
- **`(*InMemoryRateLimiter).AllowN(ctx, key, n) bool`** -- Checks if `n` requests are allowed.
- **`(*InMemoryRateLimiter).Stop()`** -- Stops the background cleanup goroutine.

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

- **Middleware depends on `clientip.NewMiddleware`.** The default `Middleware` reads `clientip.FromRequest(r).RateLimitKey` from context. If the clientip middleware has not run, the key will be empty and all requests will share a single bucket.
- **Background cleanup prevents memory leaks.** Stale buckets (unused for 10 minutes) are cleaned up every 5 minutes by a background goroutine.
- **Bucket cap bounds memory between cleanups.** Cleanup only runs every 5 minutes, so `maxBuckets` caps live buckets in the meantime; on overflow the oldest (least-recently-used) buckets are evicted to admit new keys. `bucketCount` (an `atomic.Int64`) tracks the live count; it is a **soft** bound that may transiently overshoot the cap under concurrent inserts or drift slightly from the true map size — acceptable, since the cap only needs to prevent unbounded growth.
- **`Stop()` must be called on shutdown.** Failing to call `Stop` will leak the cleanup goroutine.
- **Concurrent safety.** `sync.Map` is used for the limiter and last-access stores; no external locking is needed.

## Design Decisions

**Token bucket via `golang.org/x/time/rate`.** Token buckets allow controlled bursts (up to `burst` size) while maintaining a steady-state rate. This is appropriate for API rate limiting where short bursts are acceptable.

**Interface for swappability.** The `RateLimiter` interface allows replacing the in-memory implementation with a distributed one (Redis, etc.) for multi-instance deployments without changing middleware code.

**Composite IP key from `clientip`.** Using the composite key (all observed IPs joined with `|`) instead of a single header makes rate limiting resistant to IP spoofing via proxy headers.

**Evict-oldest on overflow, not refuse.** When the bucket cap is hit the limiter evicts the least-recently-used buckets rather than refusing to create new ones. Refusing would let an attacker who fills the map block all *new* IPs from being rate-limited at all; evicting stale entries keeps coverage for active clients.

## Testing

```bash
go test ./internal/ratelimit/...
```

Tests cover the token-bucket implementation (burst handling, per-key isolation, the `LoadOrStore` race in `getLimiter`, `cleanupOldLimiters` eviction, bucket-cap eviction of the oldest key and the bounded-map guarantee under sequential and concurrent load, and `Stop` termination), middleware behavior (allowed and blocked requests), the `MiddlewareWithKey` custom-key path and IP fallback, the `HandlerFunc` wrapper, and `UserKeyFunc` extraction.

## Dependencies

**Uses:** `internal/clientip`, `internal/logger`, `golang.org/x/time/rate`

**Used by:** `internal/api` (server middleware chain)
