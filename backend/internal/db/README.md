# db

Core database connection, shared types, sentinel errors, and helper functions for the modular DB layer.

## Files

| File | Role |
|------|------|
| `db.go` | `DB` struct wrapping `*sql.DB`, `Connect`/`ConnectWithRetry` constructors, connection pool tuning, `Close`, and escape-hatch methods (`Exec`, `QueryRow`, `Conn`) |
| `types.go` | Shared domain types used across sub-packages: `SessionListItem`, `SessionDetail`, `SyncFileDetail`, `SessionListParams`, `SessionListResult`, `SessionFilterOptions`, `SessionShare`, `ShareWithSessionInfo`, `DeviceCode`, `SyncFileState`, `SyncSessionParams`, `SessionEventParams`, `SessionAccessType`/`SessionAccessInfo`, plus constants (`MaxAPIKeysPerUser`, `DefaultPageSize`, `MaxCustomTitleLength`) |
| `errors.go` | Sentinel errors for type-safe error checking with `errors.Is()`: session (`ErrSessionNotFound`, `ErrUnauthorized`), share (`ErrShareNotFound`, `ErrShareExpired`, `ErrForbidden`), file (`ErrFileNotFound`), user (`ErrUserNotFound`, `ErrOwnerInactive`), API key (`ErrAPIKeyNotFound`, `ErrAPIKeyLimitExceeded`, `ErrAPIKeyNameExists`), rate limit (`ErrRateLimitExceeded`), device code (`ErrDeviceCodeNotFound`), GitHub link (`ErrGitHubLinkNotFound`), TIL (`ErrTILNotFound`), password auth (`ErrInvalidCredentials`, `ErrAccountLocked`) |
| `helpers.go` | Shared helper functions exported for sub-packages: `IsInvalidUUIDError`, `IsUniqueViolation`, `ExtractRepoName`, `UnmarshalSessionGitInfo`, `LoadSessionSyncFiles` |

## Sub-Package Index

| Package | Import Alias | Role |
|---------|-------------|------|
| `db/session` | `dbsession` | Session CRUD, list/paginate, sync operations |
| `db/access` | `dbaccess` | Session access checks and sharing (create/list/revoke) |
| `db/dbauth` | (none needed) | OAuth, password auth, web sessions, API keys, device codes |
| `db/user` | `dbuser` | User CRUD, admin operations |
| `db/github` | `dbgithub` | GitHub link CRUD |
| `db/til` | `dbtil` | TIL CRUD |
| `db/events` | `dbevents` | Session event insertion |

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

Integration tests use `testutil.SetupTestEnvironment(t)` which spins up containerized Postgres and MinIO via Docker/Orbstack. See `connect_test.go` for connection-level tests.

## Dependencies

- `database/sql`, `github.com/jackc/pgx/v5/stdlib` -- PostgreSQL driver
- `github.com/ConfabulousDev/confab-web/internal/logger` -- Structured logging for retry warnings
- `encoding/json` -- Git info unmarshalling
