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

- **`NewMiddleware(trustedHeaders []string) func(http.Handler) http.Handler`** -- Returns middleware that extracts client IPs, overwrites `r.RemoteAddr` with the primary IP, and stores `Info` in context. `trustedHeaders` restricts which proxy headers are honored; pass `nil`/empty to trust all known headers (default). Header names are matched case-insensitively (HTTP-canonicalized).
- **`FromContext(ctx context.Context) Info`** -- Retrieves `Info` from context. Returns zero value if middleware has not run.
- **`FromRequest(r *http.Request) Info`** -- Convenience wrapper around `FromContext`.

## Trust model

By default the middleware honors every proxy header it knows about. If the
server is not fronted by a proxy that strips these headers, an attacker can
spoof them to forge their client IP and evade IP-based rate limits. Operators
close this by setting the `TRUSTED_PROXY_HEADERS` env var (parsed in
`internal/api/server.go` and passed to `NewMiddleware`) to only the header their
edge proxy sets — e.g. `Fly-Client-IP` on Fly.io, `CF-Connecting-IP` on
Cloudflare. Any header not in the allowlist (including `X-Forwarded-For`) is
ignored even when present.

## How to Extend

### Adding support for a new proxy header

1. Add the header name to the `proxyHeaders` priority list in `middleware.go` (or, for a multi-value header like `X-Forwarded-For`, add a dedicated block in `extract`). Each entry is gated by the `isTrusted` check automatically.
2. Update the doc comment on `NewMiddleware` to include the new header in the priority list.
3. Add test cases in `middleware_test.go`.

## Invariants

- **Primary IP priority order.** Fly-Client-IP > CF-Connecting-IP > True-Client-IP > X-Real-IP > X-Forwarded-For[0] > RemoteAddr. The first non-empty header wins.
- **RateLimitKey is deterministic.** All observed IPs (from headers and RemoteAddr) are collected into a set, sorted, and joined with `|`. This ensures the same combination of IPs always produces the same key.
- **RemoteAddr is always included.** The TCP-level `RemoteAddr` anchors the rate limit key so that even if all headers are spoofed, the key is still unique per TCP connection.
- **RemoteAddr is overwritten.** After extraction, `r.RemoteAddr` is replaced with the primary IP so downstream code that reads `RemoteAddr` directly gets the right value.

## Design Decisions

**Composite rate limit key instead of single IP.** A single header-based IP can be spoofed. By combining all observed IPs into a composite key, the rate limiter is resilient to header manipulation while still grouping requests from the same real client.

**Platform-agnostic header support.** The middleware checks headers from Fly.io, Cloudflare, Akamai, and nginx so the application works behind any of these proxies without configuration changes.

**Trust all headers by default, allowlist opt-in.** With no `TRUSTED_PROXY_HEADERS` configured the middleware checks all headers unconditionally — in production, only the headers set by the actual proxy are normally present, so no config is required for the common case. Operators who cannot guarantee their edge strips spoofable headers set `TRUSTED_PROXY_HEADERS` to restrict the set, trading zero-config for spoofing resistance.

## Testing

```bash
go test ./internal/clientip/...
```

Tests cover each header individually, combined headers, IPv6 address parsing, and the `extractIPFromAddr` helper.

## Dependencies

**Uses:** (standard library only)

**Used by:** `internal/ratelimit` (reads `RateLimitKey` from context), `internal/auth` (reads primary IP), `internal/api` (middleware chain setup, Fly.io logger)
