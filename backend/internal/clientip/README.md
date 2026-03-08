# clientip

Middleware for extracting real client IPs from reverse proxy headers in a platform-agnostic way.

## Files

| File | Role |
|------|------|
| `middleware.go` | Middleware, context helpers, and IP extraction logic |
| `middleware_test.go` | Tests for header extraction, combined headers, IPv6 parsing, and `extractIPFromAddr` |

## Key Types

- **`Info`** -- Contains `Primary` (single most-trusted IP for logging/display) and `RateLimitKey` (composite of all observed IPs, pipe-delimited and sorted, for anti-spoofing rate limiting).

## Key API

- **`Middleware(next http.Handler) http.Handler`** -- Extracts client IPs, overwrites `r.RemoteAddr` with the primary IP, and stores `Info` in context.
- **`FromContext(ctx context.Context) Info`** -- Retrieves `Info` from context. Returns zero value if middleware has not run.
- **`FromRequest(r *http.Request) Info`** -- Convenience wrapper around `FromContext`.

## How to Extend

### Adding support for a new proxy header

1. Add a new block in the `extract` function in `middleware.go`, following the existing pattern: read the header, add to `allIPs`, and set `primary` if it is still empty (respecting priority order).
2. Update the doc comment on `Middleware` to include the new header in the priority list.
3. Add test cases in `middleware_test.go`.

## Invariants

- **Primary IP priority order.** Fly-Client-IP > CF-Connecting-IP > True-Client-IP > X-Real-IP > X-Forwarded-For[0] > RemoteAddr. The first non-empty header wins.
- **RateLimitKey is deterministic.** All observed IPs (from headers and RemoteAddr) are collected into a set, sorted, and joined with `|`. This ensures the same combination of IPs always produces the same key.
- **RemoteAddr is always included.** The TCP-level `RemoteAddr` anchors the rate limit key so that even if all headers are spoofed, the key is still unique per TCP connection.
- **RemoteAddr is overwritten.** After extraction, `r.RemoteAddr` is replaced with the primary IP so downstream code that reads `RemoteAddr` directly gets the right value.

## Design Decisions

**Composite rate limit key instead of single IP.** A single header-based IP can be spoofed. By combining all observed IPs into a composite key, the rate limiter is resilient to header manipulation while still grouping requests from the same real client.

**Platform-agnostic header support.** The middleware checks headers from Fly.io, Cloudflare, Akamai, and nginx so the application works behind any of these proxies without configuration changes.

**No configuration required.** The middleware checks all headers unconditionally. In production, only the headers set by the actual proxy will be present. This avoids requiring operators to specify which proxy they use.

## Testing

```bash
go test ./internal/clientip/...
```

Tests cover each header individually, combined headers, IPv6 address parsing, and the `extractIPFromAddr` helper.

## Dependencies

**Uses:** (standard library only)

**Used by:** `internal/ratelimit` (reads `RateLimitKey` from context), `internal/auth` (reads primary IP), `internal/api` (middleware chain setup, Fly.io logger)
