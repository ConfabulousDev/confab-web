# user

User CRUD and admin operations: lookup, listing with stats, status management, and deletion.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct definition and OpenTelemetry tracer |
| `user.go` | All user operations: `GetUserByID`, `CountUsers`, `UserExistsByEmail`, `ListAllUsers`, `UpdateUserStatus`, `DeleteUser`, `HasOwnSessions`, `HasAPIKeys`, `GetUserSessionIDs` |

## Key API

- **`GetUserByID(ctx, userID)`** -- Returns a `models.User` or `ErrUserNotFound`.
- **`ListAllUsers(ctx)`** -- Returns all users with admin stats (session count, last API key usage, last login). Used by the admin panel.
- **`UpdateUserStatus(ctx, userID, status)`** -- Sets a user's status to active or inactive. Returns `ErrUserNotFound` if the user does not exist.
- **`DeleteUser(ctx, userID)`** -- Permanently deletes a user. Cascading foreign keys handle associated sessions, shares, keys, etc. S3 objects must be deleted separately before calling this.
- **`GetUserSessionIDs(ctx, userID)`** -- Returns all session UUIDs for a user. Used to enumerate S3 objects for cleanup before user deletion.
- **`HasOwnSessions(ctx, userID)` / `HasAPIKeys(ctx, userID)`** -- Existence checks used by admin UI to show warnings before destructive operations.
- **`CountUsers(ctx)` / `UserExistsByEmail(ctx, email)`** -- Simple lookup helpers.

## How to Extend

1. **New user field**: Add to `models.User`, update the SELECT and Scan calls in `GetUserByID` and `ListAllUsers`.
2. **New admin stat**: Add to `models.AdminUserStats`, update the aggregate query in `ListAllUsers`.
3. **New pre-deletion check**: Add a `Has*` method following the pattern of `HasOwnSessions`/`HasAPIKeys`.

## Invariants

- `DeleteUser` relies on PostgreSQL `ON DELETE CASCADE` for all associated data. The only exception is S3 objects, which must be cleaned up separately before calling `DeleteUser`.
- `UpdateUserStatus` returns `ErrUserNotFound` when 0 rows are affected (not a silent no-op).
- `ListAllUsers` uses LEFT JOINs and GROUP BY to compute stats without excluding users who have no sessions/keys/logins.

## Design Decisions

- **Admin stats in a single query**: `ListAllUsers` joins sessions, API keys, and web sessions in one query with aggregates rather than making N+1 queries per user.
- **S3 cleanup separation**: S3 object deletion is the caller's responsibility because it requires the storage client, which the DB layer does not depend on. `GetUserSessionIDs` provides the necessary data for the caller to enumerate and delete S3 objects.
- **No soft-delete**: `DeleteUser` is a hard delete. Deactivation is handled separately via `UpdateUserStatus` with the `inactive` status.

## Testing

- Integration tests: `user_test.go` (CRUD operations), `user_admin_test.go` (admin listing, status updates, deletion)
- Tests use `testutil.SetupTestEnvironment(t)` for containerized Postgres.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for types and sentinel errors
- `github.com/ConfabulousDev/confab-web/internal/models` -- `User`, `AdminUserStats`, `UserStatus` types
- `go.opentelemetry.io/otel` -- Distributed tracing
