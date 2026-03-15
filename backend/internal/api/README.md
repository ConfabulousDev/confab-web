# api

HTTP API layer for Confab. Defines all routes, middleware, and request handlers using the chi router.

## Files

| File | Role |
|------|------|
| `server.go` | `Server` struct, `NewServer`, `SetupRoutes` (full route tree), middleware chain, body size limits, timeout constants, `respondJSON`/`respondError` helpers, SPA static file serving, security headers, www redirect, CSRF setup |
| `sync.go` | Sync endpoints for CLI uploads: `POST /api/v1/sync/init`, `POST /api/v1/sync/chunk`, `POST /api/v1/sync/event`, `GET /api/v1/sessions/{id}/sync/file`, `PATCH /api/v1/sessions/{external_id}/summary`. Handles chunk continuity validation, S3 upload, timestamp/PR-link extraction from JSONL lines, incremental file reads with line_offset |
| `sessions_view.go` | Session view endpoints: `GET /api/v1/sessions` (paginated list with server-side filtering), `GET /api/v1/sessions/{id}` (canonical access), `GET /api/v1/sessions/by-external-id/{external_id}` (lookup), `PATCH /api/v1/sessions/{id}/title` (custom title) |
| `sessions.go` | Session title extraction helpers: `extractSessionTitle`, `sanitizeTitleText`, `extractTextFromMessage` for parsing JSONL transcript content |
| `analytics.go` | Analytics endpoints: `GET /api/v1/sessions/{id}/analytics` (cached card computation with smart recap LLM generation), `POST /api/v1/sessions/{id}/analytics/smart-recap/regenerate` (owner-only). Handles cache hit/miss, streaming agent file downloads, quota enforcement, and lock-based concurrency control |
| `trends.go` | `GET /api/v1/trends` -- aggregated analytics across sessions for the authenticated user, with date range and repo filtering |
| `org_analytics.go` | `GET /api/v1/org/analytics` -- per-user aggregated analytics across all users (feature-flagged via `ENABLE_ORG_ANALYTICS`) |
| `shares.go` | Share endpoints: `POST /api/v1/sessions/{id}/share`, `GET /api/v1/sessions/{id}/shares`, `GET /api/v1/shares`, `DELETE /api/v1/shares/{shareID}`. Handles recipient validation, email invitations, share URL construction |
| `keys.go` | API key management: `POST /api/v1/keys`, `GET /api/v1/keys`, `DELETE /api/v1/keys/{id}` |
| `user.go` | `GET /api/v1/me` -- returns authenticated user info with onboarding status (`has_own_sessions`, `has_api_keys`) |
| `deletes.go` | `DELETE /api/v1/sessions/{id}` -- deletes session from S3 and database (owner-only) |
| `github_links.go` | GitHub link CRUD: `POST /api/v1/sessions/{id}/github-links`, `GET /api/v1/sessions/{id}/github-links`, `DELETE /api/v1/sessions/{id}/github-links/{linkID}`. Also contains `ParseGitHubURL` and `extractPRLinkFromLine` for transcript-based PR link extraction |
| `tils.go` | TIL endpoints: `POST /api/v1/sessions/{id}/tils`, `GET /api/v1/sessions/{id}/tils`, `GET /api/v1/tils`, `PATCH /api/v1/tils/{tilID}`, `DELETE /api/v1/tils/{tilID}` |
| `access.go` | `CheckCanonicalAccess`, `RequireCanonicalRead`, and `RespondCanonicalAccessError` -- shared canonical access control logic (CF-132) used by session detail, sync file read, analytics, GitHub links, and TIL endpoints |
| `auth_config.go` | `GET /api/v1/auth/config` -- public endpoint returning enabled auth providers and feature flags |
| `client_errors.go` | `POST /api/v1/client-errors` -- accepts frontend error reports for server-side logging/observability |
| `compression.go` | `decompressMiddleware` -- handles zstd decompression of request bodies from CLI uploads |
| `content_type.go` | `validateContentType` middleware -- enforces `application/json` Content-Type on POST/PUT/PATCH requests within `/api/v1` |
| `flylogger.go` | `FlyLogger` middleware and `ParseCLIUserAgent` -- structured HTTP request logging with client IP, user ID, Fly.io region, CLI version, and 4xx error body capture |
| `tracing.go` | `SpanEnricher` middleware -- adds CLI version/OS/arch attributes to OpenTelemetry spans |
| `debug_logging.go` | `debugLoggingMiddleware` -- logs full request/response bodies when debug logging is enabled (truncated to 10KB) |

## Key Types

- **`Server`** -- holds all dependencies (DB, S3 storage, OAuth config, email service, rate limiters, feature flags). Created by `NewServer`, routes configured by `SetupRoutes`.
- **`CanonicalAccessResult`** -- result of `CheckCanonicalAccess`: viewer identity, access type (owner/recipient/system/public/none), and session detail.
- **`SmartRecapConfig`** -- configuration for LLM-powered smart recap generation (API key, model, quota, lock timeout).
- **Request/Response types** -- `SyncInitRequest`, `SyncChunkRequest`, `SyncEventRequest`, `CreateShareRequest`, `CreateAPIKeyRequest`, etc. Each handler file defines its own request/response structs.

## How to Extend

### Adding a new endpoint

1. **Create or edit the handler file.** Group related handlers in one file (e.g., `shares.go` for all share endpoints). Define request/response structs at the top.

2. **Write the handler function.** Follow the pattern: `func HandleFoo(database *db.DB) http.HandlerFunc` (closure that captures dependencies). Inside:
   - Extract user ID with `auth.GetUserID(r.Context())`
   - Parse URL params with `chi.URLParam(r, "id")`
   - Create a timeout context: `ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)`
   - Call the appropriate DB store
   - Use `respondJSON(w, status, data)` or `respondError(w, status, msg)`

3. **Register the route in `SetupRoutes`** (`server.go`). Place it in the correct auth group:
   - `auth.RequireAPIKey` group -- for CLI endpoints (no CSRF)
   - `csrfMiddleware` + `auth.RequireSession` group -- for web dashboard endpoints
   - `csrfWhenSession` + `auth.RequireSessionOrAPIKey` group -- for endpoints used by both CLI and web
   - `auth.OptionalAuth` group -- for endpoints supporting unauthenticated access (public shares)

4. **Wrap with `withMaxBody`** using the appropriate size constant (`MaxBodyXS` through `MaxBodyXL`).

5. **Add tests** -- integration tests go in `*_http_integration_test.go` files using `testutil.SetupTestEnvironment(t)`.

### Adding a new middleware

Add it to the middleware chain in `SetupRoutes`. Order matters -- see the numbered comments in the chain. Global middleware goes at the router level; route-specific middleware goes in the appropriate `r.Group` or `r.Route`.

## Invariants

- **All endpoints have body size limits.** Every route is wrapped with `withMaxBody(limit, handler)`. The t-shirt sizes are: XS (2KB), S (16KB), M (128KB), L (2MB), XL (16MB).
- **All database operations use timeout contexts.** `DatabaseTimeout` (5s) for DB queries, `StorageTimeout` (30s) for S3 operations. Never pass the raw request context to DB calls.
- **CSRF protection is applied to all web session routes.** API key routes skip CSRF because the `Authorization` header cannot be set cross-origin without CORS approval. The `csrfWhenSession` wrapper handles hybrid endpoints.
- **Content-Type validation is enforced on all `/api/v1` POST/PUT/PATCH requests.** Must be `application/json`.
- **Auth checks happen in middleware, not handlers.** Handlers assume `auth.GetUserID(ctx)` will work if reached. The exception is `OptionalAuth` routes where handlers check the return value.
- **Canonical access model (CF-132) is the single access control path for session data.** All session read endpoints (detail, sync file, analytics, GitHub links) go through `CheckCanonicalAccess`, which checks owner > recipient > system > public > none.
- **JSON responses always use `respondJSON`** which sets `Content-Type: application/json` and `Cache-Control: no-store`.
- **Errors use `respondError`** which returns `{"error": "message"}` format.
- **Rate limiting is layered.** Global limiter (100 req/s) applies to all requests. Auth endpoints get a stricter limiter (1 req/s burst 30). Uploads are rate-limited per user ID (not IP). Validation and client error endpoints have their own limiters.
- **Compression is Brotli-preferred, gzip-fallback.** Both at level 5. Applied globally via middleware.

## Design Decisions

- **chi router** -- chosen for its lightweight middleware chain, URL parameter extraction, and route grouping. No framework magic.
- **Handler closures over methods** -- most handlers are `func HandleFoo(db *db.DB) http.HandlerFunc` rather than `Server` methods. This allows passing only the dependencies each handler needs. `Server` methods are used when multiple dependencies are needed (sync, analytics).
- **Per-provider OAuth handlers** -- GitHub, Google, and OIDC callbacks are separate functions rather than a generic OAuth handler. This is intentional: each provider has subtleties (email verification, username fallbacks, OIDC discovery) that make a generic abstraction more complex than the duplication.
- **Inline HTML for auth pages** -- device verification and account deletion pages use inline HTML rather than templates. These are simple, rarely-changing pages where avoiding template dependencies simplifies deployment.
- **Smart recap lock-based concurrency** -- LLM generation uses a database lock row to prevent concurrent generation for the same session, with configurable timeout for stale lock recovery.
- **Self-healing chunk counts** -- the sync file read endpoint corrects stale DB chunk counts by comparing against actual S3 object counts on full reads.

## Testing

- **Unit tests** -- `*_test.go` files for pure logic (compression, CSRF, auth config, body size limits, GitHub URL parsing, shared session privacy filtering).
- **Integration tests** -- `*_http_integration_test.go` files that spin up a real HTTP server with containerized Postgres and MinIO. These test full request/response cycles including auth, database, and storage.
- Run with `cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/api/...`
- Use `-short` to skip integration tests during development.

## Dependencies

- **`github.com/go-chi/chi/v5`** -- HTTP router and middleware
- **`github.com/go-chi/cors`** -- CORS middleware
- **`filippo.io/csrf/gorilla`** -- CSRF protection
- **`github.com/andybalholm/brotli`** -- Brotli compression encoder
- **`github.com/klauspost/compress/zstd`** -- zstd decompression for CLI uploads
- **`go.opentelemetry.io/otel`** -- OpenTelemetry span enrichment
- **`internal/auth`** -- authentication middleware and API key generation
- **`internal/db`** (and sub-packages) -- database access layer
- **`internal/storage`** -- S3 storage for transcript chunks
- **`internal/analytics`** -- analytics computation and caching
- **`internal/recapquota`** -- smart recap per-user quota tracking
- **`internal/models`** -- shared domain types (`User`, `APIKey`, `GitHubLink`, etc.)
- **`internal/ratelimit`** -- rate limiting middleware
- **`internal/logger`** -- structured logging
- **`internal/clientip`** -- client IP extraction
- **`internal/email`** -- share invitation emails
- **`internal/validation`** -- input validation helpers
- **`internal/admin`** -- admin panel handlers (mounted at `/admin`)
