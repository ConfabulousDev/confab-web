# db

Core database connection, shared types, sentinel errors, and helper functions for the modular DB layer.

## Files

| File | Role |
|------|------|
| `db.go` | `DB` struct wrapping `*sql.DB`, `Connect`/`ConnectWithRetry` constructors, connection pool tuning, `Close`, and escape-hatch methods (`Exec`, `QueryRow`, `Conn`) |
| `types.go` | Shared domain types used across sub-packages: `SessionListItem`, `SessionDetail`, `SyncFileDetail`, `SessionListParams`, `SessionListResult`, `SessionFilterOptions`, `SessionShare`, `ShareWithSessionInfo`, `DeviceCode`, `SyncFileState`, `SyncSessionParams`, `SessionEventParams`, `SessionAccessType`/`SessionAccessInfo`, plus constants (`MaxAPIKeysPerUser`, `DefaultPageSize`, `MaxCustomTitleLength`) |
| `errors.go` | Sentinel errors for type-safe error checking with `errors.Is()`: session (`ErrSessionNotFound`, `ErrUnauthorized`), share (`ErrForbidden`), file (`ErrFileNotFound`), user (`ErrUserNotFound`, `ErrOwnerInactive`), API key (`ErrAPIKeyNotFound`, `ErrAPIKeyLimitExceeded`, `ErrAPIKeyNameExists`), device code (`ErrDeviceCodeNotFound`), GitHub link (`ErrGitHubLinkNotFound`), password auth (`ErrInvalidCredentials`, `ErrAccountLocked`), TIL (`ErrTILNotFound`), Codex rollout (`ErrRolloutNotFound`) |
| `helpers.go` | Shared helper functions exported for sub-packages: `IsInvalidUUIDError`, `IsUniqueViolation`, `ExtractRepoName` (owner/repo from a git URL, used for the per-session display field), `UnmarshalSessionGitInfo`, `LoadSessionSyncFiles` |
| `repo_filter.go` | SQL fragment helpers for repo extraction + read-time fork→upstream resolution: `RepoRootExpr(alias)` (SELECT projection) and `RepoMatchExpr(alias, paramPlaceholder)` (WHERE clause). `RepoRootExpr` resolves a session's upstream live from its own `git_info` (`repo_url` + `remotes` + `tracking_remote`) — no stored or shared mapping. Folds CF-509 trailing-slash handling into the extraction regex. One source of truth across the call sites that filter sessions by `owner/repo` (CF-510). |
| `visibility.go` | CF-495 SQL CTE helper `VisibleSessionsCTE(shareAllSessions)` returning `visible_sessions(id, user_id, owner_email, access_type, shared_by_email)` for the session-visibility predicate. Single source of truth used by analytics (`trends.go`), session-list pagination (`db/session/session.go`), and filter-options paths (`db/session`, `db/til`). UNION ALL — callers wrap with `SELECT DISTINCT` (analytics) or `DISTINCT ON (id)` priority dedup (pagination). |

## Sub-Package Index

| Package | Import Alias | Role |
|---------|-------------|------|
| `db/session` | `dbsession` | Session CRUD, list/paginate, sync operations |
| `db/access` | `dbaccess` | Session access checks and sharing (create/list/revoke) |
| `db/dbauth` | (none needed) | OAuth, password auth, web sessions, API keys, device codes |
| `db/user` | `dbuser` | User CRUD, admin operations |
| `db/github` | `dbgithub` | GitHub link CRUD |
| `db/til` | `dbtil` | TIL CRUD |
| `db/dbadminsettings` | (none needed) | Admin settings key-value store |
| `db/events` | `dbevents` | Session event insertion |
| `db/codex` | `dbcodex` | Codex rollout sidecar (parent-child thread tree, recursive CTE) |

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
- **`SessionDetail.RedactForSharing()`** -- Strips PII fields (hostname, username, cwd, transcript path) for non-owner access.

## How to Extend

1. **Adding a new sub-package**: Create `db/newpkg/`, define `Store` with `DB *db.DB`, add a `conn()` helper. Add shared types/errors to this root package.
2. **Adding a new sentinel error**: Add to `errors.go` and use `errors.Is()` for checking.
3. **Adding shared helpers**: Put in `helpers.go` with an exported name. Sub-packages import and call them.
4. **New shared types**: Add to `types.go`. Sub-packages should never define types that are consumed across package boundaries.

## Invariants

- The `DB.conn` field is private; sub-packages access it via `DB.Conn()`.
- `ShareAllSessions` bypasses share-row checks -- every authenticated user gets system-level access.
- `SessionDetail.RedactForSharing()` must be called for all non-owner session access to strip PII.
- Sentinel errors are the contract between DB layer and HTTP handlers; never return raw SQL errors to callers.

## Design Decisions

- **Modular sub-packages over monolith**: The DB layer was split from a single large package into domain-focused sub-packages to improve code organization and reduce coupling.
- **`*sql.DB` exposed via `Conn()`**: Sub-packages need the raw connection for `QueryContext`/`ExecContext`. The `DB` wrapper adds pool config and the `ShareAllSessions` flag but otherwise stays thin.
- **Connection pool tuning**: 500 max open / 100 max idle / 20-minute max lifetime. Tuned for a multi-tenant web backend with bursty sync traffic.
- **`ConnectWithRetry` with exponential backoff**: Allows the server to start before the database is fully ready (useful in container orchestration).
- **pgx stdlib driver**: Uses `pgx/v5/stdlib` for compatibility with `database/sql` while getting pgx performance.

## Testing

- Unit tests: `helpers_test.go` (`ExtractRepoName`, `IsInvalidUUIDError`, `IsUniqueViolation`, `UnmarshalSessionGitInfo`), `redaction_test.go` (`SessionDetail.RedactForSharing` field completeness via reflection).
- Integration tests: `helpers_integration_test.go` (`LoadSessionSyncFiles` happy path + todo exclusion + empty result, plus a `Connect`/`Exec`/`QueryRow`/`Conn` lifecycle check) and `connect_test.go` (`ConnectWithRetry` context cancellation). All integration tests use `testutil.SetupTestEnvironment(t)`, which spins up containerized Postgres and MinIO via Docker/Orbstack.

## Dependencies

- `database/sql`, `github.com/jackc/pgx/v5/stdlib` -- PostgreSQL driver
- `github.com/ConfabulousDev/confab-web/internal/logger` -- Structured logging for retry warnings
- `encoding/json` -- Git info unmarshalling
