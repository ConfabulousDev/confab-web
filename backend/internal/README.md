# Backend Package Index

Internal packages for the Confab backend server. All packages live under
`backend/internal/` and are not importable by external Go modules.

## Package Index

| Package | Purpose | Change this when... |
|---------|---------|---------------------|
| `admin` | Super-admin handlers (user management, system shares, settings, audit) | Adding admin actions, changing admin authorization rules, adding admin settings |
| `analytics` | Session analytics computation, card storage, trends, search index, smart recaps | Adding analytics cards, changing cost/token calculations, modifying analyzers |
| `anthropic` | HTTP client for the Anthropic Messages API | Changing AI model calls, updating API version |
| `api` | HTTP handlers, routing (chi), middleware wiring, request/response helpers | Adding/changing API endpoints, adjusting rate limits, modifying middleware stack |
| `auth` | Authentication middleware, OAuth flows (GitHub/Google/OIDC), password auth, API key validation | Adding auth providers, changing session/token logic |
| `clientip` | Middleware to extract real client IPs (Fly.io, Cloudflare, nginx) | Supporting new reverse proxy headers |
| `codex` | Parser for OpenAI Codex CLI rollout JSONL — `ParseRollout` returns a normalized `ParsedRollout` (turns, tool calls, token usage, compactions) consumed by analytics | Changing how Codex rollouts are interpreted, adding new event types |
| `db` | Database connection, shared types (`SessionListItem`, `SessionDetail`), error sentinels, helpers | Changing connection pooling, adding shared DB types |
| `db/access` | Session access checks and share CRUD | Changing share permissions, access control rules |
| `db/codex` | Codex rollout sidecar store (`codex_rollouts` table): `UpsertRollout`, `GetRollout`, `ListSubtree` recursive CTE. Records the parent-child thread tree without modifying `sessions` | Changing Codex parent-child thread storage, adding sidecar fields |
| `db/dbadmincardinvalidations` | Admin card invalidation audit table + smart-recap quota-bypass signal (CF-343) | Changing card invalidation semantics, audit shape |
| `db/dbadminsettings` | Admin settings key-value store (`admin_settings` table) | Adding new admin-configurable settings |
| `db/dbauth` | OAuth accounts, password hashes, web sessions, API keys, device codes | Adding auth storage, changing token/session schema |
| `db/events` | Session event insertion (e.g., sync events) | Adding new event types |
| `db/github` | GitHub link CRUD | Changing GitHub integration storage |
| `db/migrations` | Embedded SQL migration files | Adding schema changes (new tables, columns, indexes) |
| `db/session` | Session CRUD, list/paginate, sync, full-text search | Changing session queries, filters, pagination |
| `db/til` | TIL CRUD | Changing TIL storage or queries |
| `db/user` | User CRUD, admin user listing | Changing user schema, adding user fields |
| `email` | Email service interface + Resend implementation (share invitations) | Adding email types, changing email provider |
| `httputil` | HTTP response helpers shared between `api` and `admin` (e.g., `RespondJSON`) | Adding new shared response/render helpers |
| `logger` | Structured JSON logging (slog), request-scoped context logger | Changing log format, adding log fields |
| `models` | Domain types shared across packages (`User`, `OAuthProvider`) and provider identity (`ProviderClaudeCode`, `ProviderCodex`, `LegacyAliases`, `AllowedProviders`, `NormalizeProvider`, `ExpandWithAliases` in `provider.go`) | Adding domain-wide types, adding/renaming a provider, or registering a permanent legacy alias |
| `pricingsource` | Owns the model price table: the embedded `pricing.json` (single source of truth) + a lazy, best-effort refresh from confabulous.dev with freshest-wins fallback. Serves `/api/v1/pricing`; feeds analytics cost compute | Changing the price data, the document schema, TTLs, the source URL, or the fallback semantics |
| `ratelimit` | Rate limiter interface + in-memory token bucket implementation | Changing rate limit strategies, adding distributed limiter |
| `recapquota` | Per-user monthly smart recap quota tracking | Changing quota rules, billing logic |
| `storage` | MinIO/S3 client, chunk operations (download, merge, parse keys) | Changing object storage, chunk format |
| `testutil` | Test helpers: Docker containers (Postgres/MinIO), test server, fixtures | Adding test infrastructure, changing test patterns |
| `updatecheck` | Lazy GitHub-release fetch + TTL cache; reports whether the running backend is behind the latest release for the in-product "Update available" badge on `/api/v1/auth/config` | Changing the GitHub source, TTLs, semver semantics, or the response shape |
| `validation` | Input validation (email normalization, field size limits, external ID) | Adding validation rules, changing DB constraints |

## Dependency Map

Arrows point from **importer** to **dependency**. Leaf packages at the bottom
have no internal dependencies.

```
  api          ─→ admin, auth, analytics, ratelimit, email,
                  storage, db/*, models, recapquota, validation,
                  clientip, logger

  admin        ─→ analytics, auth, db, db/access, db/dbadmincardinvalidations,
                  db/dbadminsettings, db/dbauth, db/user, models, recapquota,
                  storage, validation, logger

  auth         ─→ db, db/dbauth, db/user, models,
                  clientip, logger, validation

  analytics    ─→ codex, storage, anthropic, db, db/dbadminsettings, recapquota

  ratelimit    ─→ clientip, logger

  email        ─→ db, logger  (for provider normalisation + ERROR log
                               on unknown providers in share invitations)

  db/access                    ┐
  db/codex                     │
  db/dbadmincardinvalidations  │ (also imports analytics for
  db/dbauth                    │  AllCardTableNames validation)
  db/events                    ├─→ db (root only; sub-packages do NOT
  db/github                    │     import each other)
  db/session                   │
  db/til                       │
  db/user                      ┘

  Leaf packages (zero internal deps):
    clientip, logger, validation, models, anthropic,
    recapquota, storage, codex

  Test-only:
    testutil   ─→ db, db/migrations, storage, auth, models
```

### Import Aliases

The codebase uses consistent import aliases for `db` sub-packages:

```go
dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
dbaccess   "github.com/ConfabulousDev/confab-web/internal/db/access"
dbuser     "github.com/ConfabulousDev/confab-web/internal/db/user"
dbgithub   "github.com/ConfabulousDev/confab-web/internal/db/github"
dbtil      "github.com/ConfabulousDev/confab-web/internal/db/til"
dbevents   "github.com/ConfabulousDev/confab-web/internal/db/events"
// dbauth has no alias — package name is already "dbauth"
```

## Data Flow

How a request flows through the system, from HTTP to storage and back:

```
Client (browser / CLI)
  │
  ▼
┌─────────────────────────────────────────────────────┐
│  api.SetupRoutes()  — chi router                    │
│                                                     │
│  Middleware chain (in order):                        │
│  1. Recoverer (panic recovery)                      │
│  2. ClientIP (extract real IP from proxy headers)   │
│  3. RateLimit (reject abusive requests early)       │
│  4. RequestID                                       │
│  5. SpanEnricher (OpenTelemetry)                    │
│  6. Logger (request-scoped structured logging)      │
│  7. Redirects + Security headers                    │
│  8. Compression (Brotli / gzip)                     │
│  9. FlyLogger                                       │
│  10. CORS                                           │
│  11. CSRF (session-based routes only)               │
│  12. Auth (RequireSession / RequireAPIKey /          │
│          OptionalAuth — per route group)             │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
              ┌────────────────┐
              │  HTTP Handler  │  e.g., HandleGetSession
              │   (api pkg)    │
              └───────┬────────┘
                      │
        ┌─────────────┼─────────────┐
        v             v             v
  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │    db    │  │ analytics│  │ storage  │
  │ (SQL)   │  │ (compute)│  │ (S3/MinIO│
  └────┬─────┘  └─────┬────┘  └─────┬────┘
       │              │              │
       v              v              v
   PostgreSQL    Anthropic API   MinIO / S3
```

### Key Request Paths

| Path | Handler → Dependencies |
|------|----------------------|
| `GET /api/v1/sessions` | `api` → `auth` → `db/session` (list + paginate) |
| `GET /api/v1/sessions/{id}` | `api` → `auth` (optional) → `db/access` (access check) → `db/session` (detail) |
| `POST /api/v1/sync/chunk` | `api` → `auth` (API key) → `db/session` (upsert) → `storage` (S3 upload) |
| `GET /api/v1/sessions/{id}/analytics` | `api` → `auth` → `db/access` → `analytics` (compute/cache) → `storage` (JSONL download) |
| `POST /auth/github/callback` | `auth` (OAuth) → `db/dbauth` (upsert OAuth account) → `db/user` (find/create user) |
| `GET /admin/users` | `api` → `auth` (session) → `admin` (middleware + handlers) → `db/user` |

## Layering Rules

1. **`api` and `admin`** are the top-level HTTP layers. They may import any other package.
2. **`auth`** handles authentication concerns. It imports `db`, `db/dbauth`, `db/user`, `models`, `clientip`, `logger`, `validation`.
3. **`analytics`** handles computation. It imports `storage`, `anthropic`, `recapquota` but NOT `api` or `auth`.
4. **`db` sub-packages** (`access`, `codex`, `dbauth`, `events`, `github`, `session`, `til`, `user`) depend only on `db` root (for the `DB` struct and shared types). They do NOT import each other.
5. **Leaf packages** (`logger`, `clientip`, `models`, `anthropic`, `recapquota`, `storage`) have zero internal dependencies. `validation` imports `models` for the canonical provider list. `ratelimit` has minimal deps (`clientip`, `logger`). `email` has minimal deps (`logger`, `models`) — it consults `models.NormalizeProvider` plus the canonical provider constants to keep share-invitation wording in lockstep with the rest of the codebase. None of these may import `api`, `auth`, `admin`, or `analytics`.
6. **`testutil`** is test-only infrastructure. Production code must not import it.
7. **No circular imports.** If two packages need to share a type, put it in `db/types.go` or `models/models.go`.
