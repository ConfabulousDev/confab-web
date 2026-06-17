# db

Core database connection, shared types, sentinel errors, and helper functions for the modular DB layer.

## Files

| File | Role |
|------|------|
| `db.go` | `DB` struct wrapping `*sql.DB`, `Connect`/`ConnectWithRetry` constructors, connection pool tuning, `Close`, and escape-hatch methods (`Exec`, `QueryRow`, `Conn`) |
| `types.go` | Shared domain types used across sub-packages: `SessionListItem`, `SessionDetail`, `SyncFileDetail`, `SessionListParams`, `SessionListResult`, `SessionFilterOptions`, `SessionShare`, `ShareWithSessionInfo`, `DeviceCode`, `SyncFileState`, `SyncSessionParams`, `SessionEventParams`, `SessionAccessType`/`SessionAccessInfo`, plus constants (`MaxAPIKeysPerUser`, `DefaultPageSize`, `MaxCustomTitleLength`) |
| `errors.go` | Sentinel errors for type-safe error checking with `errors.Is()`: session (`ErrSessionNotFound`, `ErrUnauthorized`), share (`ErrForbidden`), file (`ErrFileNotFound`), user (`ErrUserNotFound`, `ErrOwnerInactive`), API key (`ErrAPIKeyNotFound`, `ErrAPIKeyLimitExceeded`, `ErrAPIKeyNameExists`), device code (`ErrDeviceCodeNotFound`), GitHub link (`ErrGitHubLinkNotFound`), password auth (`ErrInvalidCredentials`, `ErrAccountLocked`), Codex rollout (`ErrRolloutNotFound`) |
| `helpers.go` | Shared helper functions exported for sub-packages: `IsInvalidUUIDError`, `IsUniqueViolation`, `ExtractRepoName` (owner/repo from a git URL, used for the per-session display field), `UnmarshalSessionGitInfo`, `LoadSessionSyncFiles` |
| `tokenhash.go` | `HashToken(raw)` -- hex-encoded SHA-256, the single hashing primitive for tokens stored hashed at rest (API keys, web-session IDs, device codes). Lives here (not `auth`) so both `auth` and `db/dbauth` share it without an import cycle. No salt (high-entropy random tokens; preserves single-indexed exact-match lookup) (40hj). |
| `git_info_redact.go` | `SanitizeGitInfoForSharing(raw interface{}) interface{}` -- read-time redaction of the free-form `git_info` JSONB for non-owner access (recipient, system, public alike). Whitelists `branch` + a host/credential-stripped `owner/repo` display name; drops remote URLs, `tracking_remote`, author, and every other key. Fails safe (nil/non-map/unparseable → drop, never the original). Deliberately stricter than `ExtractRepoName`/`repo_filter.go` (which fall back to the original URL) — see the doc comment before consolidating. Called by `db/access.GetSessionDetailWithAccess` (d29s). |
| `repo_filter.go` | SQL fragment helpers for repo extraction + read-time fork→upstream resolution: `RepoRootExpr(alias)` (SELECT projection) and `RepoMatchExpr(alias, paramPlaceholder)` (WHERE clause). `RepoRootExpr` resolves a session's upstream live from its own `git_info` (`repo_url` + `remotes` + `tracking_remote`) — no stored or shared mapping. Folds CF-509 trailing-slash handling into the extraction regex. One source of truth across the call sites that filter sessions by `owner/repo` (CF-510). Also home to `ListableSessionPredicate(alias)` — the single SQL fragment defining session "listability" (synced lines > 0 AND a summary or first_user_message). The paginated session list **and** the repo/branch/owner/model filter-option queries (session-list `queryFilterOptions`, Trends `aggregateFilterOptions` + `modelFilterOptions`) all apply it, so an offered filter option can never orphan to an empty list (0407). |
| `visibility.go` | CF-495 SQL CTE helper `VisibleSessionsCTE(shareAllSessions)` returning `visible_sessions(id, user_id, owner_email, access_type, shared_by_email)` for the session-visibility predicate. Single source of truth used by analytics (`trends.go`), session-list pagination (`db/session/session.go`), and filter-options paths (`db/session`). UNION ALL — callers wrap with `SELECT DISTINCT` (analytics) or `DISTINCT ON (id)` priority dedup (pagination). |
| `tokens_v2.go` | SQL fragments that extract a session's top-level scalars from the `session_card_tokens_v2.data` JSONB (all via the private `v2DataKeyExpr(alias, key)`): `V2TotalCostExpr` (`total_cost_usd`), plus the four token-**count** accessors `V2TotalInputExpr` / `V2TotalOutputExpr` / `V2TotalCacheCreationExpr` / `V2TotalCacheReadExpr` (pjnz). One source of truth for the per-session cost/token readers that moved off the flat v1 `session_card_tokens` table: cost readers (37cg) — session list (`db/session`), org analytics + Trends costliest-sessions (`analytics`); the four-count daily time-series (pjnz) — Trends `aggregateTokens` (`analytics`). Returns nullable text — presentational LEFT-JOIN callers read it raw, aggregating INNER-JOIN callers wrap `COALESCE(<expr>, '0')::numeric` (cost) or `::bigint` (counts). |

## Sub-Package Index

| Package | Import Alias | Role |
|---------|-------------|------|
| `db/session` | `dbsession` | Session CRUD, list/paginate, sync operations |
| `db/access` | `dbaccess` | Session access checks and sharing (create/list/revoke) |
| `db/dbauth` | (none needed) | OAuth, password auth, web sessions, API keys, device codes |
| `db/user` | `dbuser` | User CRUD, admin operations |
| `db/github` | `dbgithub` | GitHub link CRUD |
| `db/dbadminsettings` | (none needed) | Admin settings key-value store |
| `db/events` | `dbevents` | Session event insertion |
| `db/codex` | `dbcodex` | Codex rollout sidecar (parent-child thread tree, recursive CTE) |
| `db/cursor` | `dbcursor` | Cursor session-metadata sidecar (per-session model name; first-non-empty-wins) |

All sub-packages follow the same `Store` struct pattern:

```go
type Store struct {
    DB *db.DB
}

func (s *Store) conn() *sql.DB { return s.DB.Conn() }
```

Each sub-package depends on the root `db` package for the `DB` handle, shared types, and sentinel errors.

## Key Types

- **`DB`** -- Wraps `*sql.DB` with a `ShareAllSessions` flag for on-prem deployments where all sessions are visible to all authenticated users.
- **`SessionAccessType`/`SessionAccessInfo`** -- Enum + struct describing how a user can access a session (owner, recipient, system, public, none) and whether authentication would help.
- **`SessionDetail.RedactForSharing()`** -- Strips PII fields (hostname, username, cwd, transcript path) for non-owner access. Does NOT touch `git_info` (it runs before the git_info unmarshal and only nils `*string` fields) — that JSONB blob is sanitized separately via `SanitizeGitInfoForSharing`.
- **`SanitizeGitInfoForSharing(raw)`** -- Redacts the unmarshaled `git_info` for non-owner access: keeps `branch` + a host/credential-free `owner/repo` display name, drops everything else (d29s).

## How to Extend

1. **Adding a new sub-package**: Create `db/newpkg/`, define `Store` with `DB *db.DB`, add a `conn()` helper. Add shared types/errors to this root package.
2. **Adding a new sentinel error**: Add to `errors.go` and use `errors.Is()` for checking.
3. **Adding shared helpers**: Put in `helpers.go` with an exported name. Sub-packages import and call them.
4. **New shared types**: Add to `types.go`. Sub-packages should never define types that are consumed across package boundaries.

## Invariants

- The `DB.conn` field is private; sub-packages access it via `DB.Conn()`.
- `ShareAllSessions` bypasses share-row checks -- every authenticated user gets system-level access.
- `SessionDetail.RedactForSharing()` must be called for all non-owner session access to strip PII. The free-form `git_info` JSONB is NOT covered by it — non-owner access must additionally run `SanitizeGitInfoForSharing`, since a raw remote URL can embed credentials. Any new `interface{}`/JSONB field on `SessionDetail` is guarded by `TestSessionDetail_InterfaceFieldsAreClassified`.
- Sentinel errors are the contract between DB layer and HTTP handlers; never return raw SQL errors to callers.

## Design Decisions

- **Modular sub-packages over monolith**: The DB layer was split from a single large package into domain-focused sub-packages to improve code organization and reduce coupling.
- **`*sql.DB` exposed via `Conn()`**: Sub-packages need the raw connection for `QueryContext`/`ExecContext`. The `DB` wrapper adds pool config and the `ShareAllSessions` flag but otherwise stays thin.
- **Connection pool tuning**: 500 max open / 100 max idle / 20-minute max lifetime. Tuned for a multi-tenant web backend with bursty sync traffic.
- **`ConnectWithRetry` with exponential backoff**: Allows the server to start before the database is fully ready (useful in container orchestration).
- **pgx stdlib driver**: Uses `pgx/v5/stdlib` for compatibility with `database/sql` while getting pgx performance.

## Testing

- Unit tests: `helpers_test.go` (`ExtractRepoName`, `IsInvalidUUIDError`, `IsUniqueViolation`, `UnmarshalSessionGitInfo`), `redaction_test.go` (`SessionDetail.RedactForSharing` field completeness + the `interface{}`/JSONB classification guard, via reflection), `git_info_redact_test.go` (`SanitizeGitInfoForSharing` whitelist + credential/host stripping across URL forms).
- Integration tests: `helpers_integration_test.go` (`LoadSessionSyncFiles` happy path + todo exclusion + empty result, plus a `Connect`/`Exec`/`QueryRow`/`Conn` lifecycle check) and `connect_test.go` (`ConnectWithRetry` context cancellation). All integration tests use `testutil.SetupTestEnvironment(t)`, which spins up containerized Postgres and MinIO via Docker/Orbstack.

## Dependencies

- `database/sql`, `github.com/jackc/pgx/v5/stdlib` -- PostgreSQL driver
- `github.com/ConfabulousDev/confab-web/internal/logger` -- Structured logging for retry warnings
- `encoding/json` -- Git info unmarshalling
