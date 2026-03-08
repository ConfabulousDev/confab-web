# admin

Server-rendered admin UI and middleware for super-admin user management.

## Files

| File | Role |
|------|------|
| `admin.go` | `IsSuperAdmin` check against the `SUPER_ADMIN_EMAILS` env var |
| `admin_test.go` | Unit tests for `IsSuperAdmin` |
| `admin_http_integration_test.go` | Integration tests for admin HTTP handlers |
| `audit.go` | Structured audit logging for all admin actions |
| `handlers.go` | HTTP handlers for user list, create, activate/deactivate, delete, and system shares |
| `middleware.go` | Chi middleware that gates routes to super admins only |

## Key Types

- **`Handlers`** -- Dependency holder (DB, Storage, config flags) for all admin HTTP handlers.
- **`AdminAction`** -- String enum (`user.create`, `user.deactivate`, `user.activate`, `user.delete`, `system_share.create`) used as audit log keys.

## Key API

- **`IsSuperAdmin(email string) bool`** -- Checks the comma-separated `SUPER_ADMIN_EMAILS` env var (case-insensitive, trimmed).
- **`Middleware(database *db.DB)`** -- Returns a `func(http.Handler) http.Handler` that rejects non-super-admins with 403.
- **`AuditLog` / `AuditLogFromRequest`** -- Logs admin actions with admin identity, action type, and arbitrary detail key-value pairs. All log lines include `"audit", true` for filtering.
- **`NewHandlers(database, store, passwordAuthEnabled, allowedDomains, sharesEnabled)`** -- Constructor that wires up dependencies.

### Handler methods on `Handlers`

| Method | Route pattern | Description |
|--------|--------------|-------------|
| `HandleListUsers` | `GET /admin/users` | Renders user table with recap stats |
| `HandleDeactivateUser` | `POST /admin/users/{id}/deactivate` | Sets user status to inactive |
| `HandleActivateUser` | `POST /admin/users/{id}/activate` | Sets user status to active |
| `HandleDeleteUser` | `POST /admin/users/{id}/delete` | Deletes user, their S3 objects, then DB record |
| `HandleCreateUserPage` | `GET /admin/users/new` | Renders password-user creation form |
| `HandleCreateUser` | `POST /admin/users/create` | Creates a password-authenticated user |
| `HandleSystemSharePage` | `GET /admin/system-shares` | Renders system share creation form |
| `HandleCreateSystemShareForm` | `POST /admin/system-shares` | Creates a system-wide share for a session. Takes an extra `frontendURL string` parameter beyond the standard `(w, r)` |

## How to Extend

### Adding a new admin action

1. Add a new `AdminAction` constant in `audit.go`.
2. Write the handler method on `Handlers` in `handlers.go`.
3. Call `AuditLogFromRequest` with the new action and relevant details.
4. Register the route in `backend/internal/api/server.go` under the admin group.

## Invariants

- **Middleware ordering.** `Middleware` must run after `auth.SessionMiddleware`; it reads the user ID from context via `auth.GetUserID`.
- **S3 before DB on delete.** `HandleDeleteUser` deletes S3 objects first, then the database row. If S3 fails, the DB row is preserved so the operation can be retried.
- **Audit logging on every mutating action.** Every state-changing handler calls `AuditLogFromRequest` before redirecting.
- **All HTML is inline.** Admin pages render HTML directly in Go (no external template files) to simplify deployment. This is intentional for low-churn internal tooling.
- **Database timeout.** All DB operations use a 5-second context timeout (`DatabaseTimeout`), except user deletion which uses 60 seconds to allow for S3 cleanup.

## Design Decisions

**Inline HTML instead of template files.** Admin pages are internal tools that change rarely. Keeping HTML inline avoids template file management and makes the binary self-contained.

**Env-var-based super admin list.** `SUPER_ADMIN_EMAILS` is read on every request rather than cached, so changes take effect without restart. The list is expected to be small (a few emails).

**Shared email normalization.** The admin package uses `validation.NormalizeEmail` when checking the super admin list, keeping email handling consistent across the codebase.

## Testing

```bash
go test ./internal/admin/...
```

Unit tests cover `IsSuperAdmin` with various env var configurations. Integration tests (`admin_http_integration_test.go`) exercise the full HTTP handler flow with a real database.

## Dependencies

**Uses:** `internal/auth`, `internal/db`, `internal/db/access`, `internal/db/dbauth`, `internal/db/user`, `internal/logger`, `internal/models`, `internal/recapquota`, `internal/storage`, `internal/validation`, `github.com/go-chi/chi/v5`, `golang.org/x/crypto/bcrypt`

**Used by:** `internal/api` (server setup and routing)
