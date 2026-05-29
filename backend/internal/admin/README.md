# admin

Admin API handlers, middleware, and audit logging for super-admin user management.

## Files

| File | Role |
|------|------|
| `admin.go` | `IsSuperAdmin` check against the `SUPER_ADMIN_EMAILS` env var |
| `admin_test.go` | Unit tests for `IsSuperAdmin` |
| `api_handlers.go` | JSON API handlers for user CRUD, activate/deactivate, delete, system shares, and smart recap prompt settings |
| `api_handlers_test.go` | Full HTTP stack integration tests for admin API endpoints |
| `audit.go` | Structured audit logging for all admin actions |
| `card_invalidations.go` | JSON API handlers for date-range card invalidation (`/admin/cards/invalidate`, `/admin/cards/invalidations`) |
| `card_invalidations_test.go` | Integration tests for the card invalidation handlers |
| `handlers.go` | `Handlers` struct and `NewHandlers` constructor (dependency holder) |
| `middleware.go` | Chi middleware that gates routes to super admins only |

## Key Types

- **`Handlers`** -- Dependency holder (DB, Storage, config flags) for all admin HTTP handlers.
- **`AdminAction`** -- String enum (`user.create`, `user.deactivate`, `user.activate`, `user.delete`, `system_share.create`, `setting.update`, `setting.reset`, `smart_recap.regenerate_all`, `cards.invalidate`) used as audit log keys.
- **`AdminUserListResponse`**, **`AdminUserJSON`**, **`AdminTotals`** -- JSON response types for the user list endpoint.
- **`SystemSharesResponse`**, **`SystemShareJSON`** -- JSON response types for system shares. `SystemShareJSON.Provider` is the canonical session provider (`"claude-code"` / `"codex"`), normalized at the DB boundary so the admin UI can render a brand chip without re-normalizing (CF-370).
- **`SmartRecapPromptResponse`**, **`SetSmartRecapPromptRequest`**, **`SetSmartRecapPromptResponse`**, **`DeleteSmartRecapPromptResponse`** -- JSON request/response types for smart recap prompt settings.
- **`InvalidateCardsRequest`**, **`InvalidateCardsResponse`**, **`CardInvalidationRow`**, **`CardInvalidationsListResponse`** -- JSON request/response types for card invalidations (CF-343).

## Key API

- **`IsSuperAdmin(email string) bool`** -- Checks the comma-separated `SUPER_ADMIN_EMAILS` env var (case-insensitive, trimmed).
- **`Middleware(database *db.DB)`** -- Returns a `func(http.Handler) http.Handler` that rejects non-super-admins with 403.
- **`AuditLog` / `AuditLogFromRequest`** -- Logs admin actions with admin identity, action type, and arbitrary detail key-value pairs. All log lines include `"audit", true` for filtering.
- **`NewHandlers(database, store, frontendURL, allowedDomains, sharesEnabled)`** -- Constructor that wires up dependencies. Internally creates `settingsStore` (`dbadminsettings.Store`) and `analyticsStore` (`analytics.Store`).

### Handler methods on `Handlers`

| Method | Route pattern | Description |
|--------|--------------|-------------|
| `HandleListUsersAPI` | `GET /api/v1/admin/users` | Returns JSON user list with recap stats and totals |
| `HandleCreateUserAPI` | `POST /api/v1/admin/users` | Creates a password-authenticated user (when password auth enabled) |
| `HandleDeactivateUserAPI` | `POST /api/v1/admin/users/{id}/deactivate` | Sets user status to inactive |
| `HandleActivateUserAPI` | `POST /api/v1/admin/users/{id}/activate` | Sets user status to active |
| `HandleDeleteUserAPI` | `DELETE /api/v1/admin/users/{id}` | Deletes user, their S3 objects, then DB record |
| `HandleListSystemSharesAPI` | `GET /api/v1/admin/system-shares` | Returns all system-wide shares |
| `HandleCreateSystemShareAPI` | `POST /api/v1/admin/system-shares` | Creates a system-wide share |
| `HandleGetSmartRecapPrompt` | `GET /api/v1/admin/settings/smart-recap-prompt` | Returns current prompt (custom or default) plus fixed sections |
| `HandleGetSmartRecapPromptDefault` | `GET /api/v1/admin/settings/smart-recap-prompt/default` | Returns the hardcoded default instructions |
| `HandleSetSmartRecapPrompt` | `PUT /api/v1/admin/settings/smart-recap-prompt` | Saves custom instructions (validates UTF-8, max 50k chars) |
| `HandleDeleteSmartRecapPrompt` | `DELETE /api/v1/admin/settings/smart-recap-prompt` | Resets to default by deleting the custom setting |
| `HandleGetSmartRecapRegenerateCount` | `GET /api/v1/admin/settings/smart-recap-prompt/regenerate-count` | Returns count of sessions with smart recap cards |
| `HandleRegenerateAllSmartRecaps` | `POST /api/v1/admin/settings/smart-recap-prompt/regenerate-all` | Triggers bulk regeneration via timestamp in `admin_settings` |
| `HandleInvalidateCards` | `POST /api/v1/admin/cards/invalidate` | Dry-run or execute DELETE of `session_card_*` rows for sessions in a date window. Writes audit rows so the smart-recap quota is bypassed on recompute. Defaults to `dry_run: true` |
| `HandleListCardInvalidations` | `GET /api/v1/admin/cards/invalidations` | Returns up to 500 recent audit rows; `?correlation_id=` filters to one run |

## How to Extend

### Adding a new admin action

1. Add a new `AdminAction` constant in `audit.go`.
2. Write the handler method on `Handlers` in `api_handlers.go`.
3. Call `AuditLogFromRequest` with the new action and relevant details.
4. Register the route in `backend/internal/api/server.go` under the admin group.

## Invariants

- **Middleware ordering.** `Middleware` must run after `auth.SessionMiddleware`; it reads the user ID from context via `auth.GetUserID`.
- **S3 before DB on delete.** `HandleDeleteUserAPI` deletes S3 objects first, then the database row. If S3 fails, the DB row is preserved so the operation can be retried.
- **Audit logging on every mutating action.** Every state-changing handler calls `AuditLogFromRequest` before responding. Payloads include enough context that the entry stays interpretable after the underlying row is deleted -- e.g. `system_share.create` records `provider` alongside `session_id` and `external_id` so the action can be attributed to Claude vs Codex even if the session row is later gone.
- **Database timeout.** All DB operations use a 5-second context timeout (`DatabaseTimeout`), except user deletion which uses 60 seconds to allow for S3 cleanup.

## Design Decisions

**JSON API instead of inline HTML.** Admin UI is a React SPA (CF-322). All handlers return JSON; the old server-rendered HTML handlers have been removed.

**Env-var-based super admin list.** `SUPER_ADMIN_EMAILS` is read on every request rather than cached, so changes take effect without restart. The list is expected to be small (a few emails).

**Shared httputil package.** JSON response helpers (`RespondJSON`, `RespondError`) live in `internal/httputil` to avoid circular imports between `api` and `admin`.

## Testing

```bash
go test ./internal/admin/...
```

Unit tests cover `IsSuperAdmin` with various env var configurations. Integration tests (`api_handlers_test.go`) exercise the full HTTP handler flow with real database, CSRF, and auth middleware.

## Dependencies

**Uses:** `internal/analytics`, `internal/auth`, `internal/db`, `internal/db/access`, `internal/db/dbadmincardinvalidations`, `internal/db/dbadminsettings`, `internal/db/dbauth`, `internal/db/user`, `internal/httputil`, `internal/logger`, `internal/models`, `internal/recapquota`, `internal/storage`, `internal/validation`, `github.com/go-chi/chi/v5`, `golang.org/x/crypto/bcrypt`

**Used by:** `internal/api` (server setup and routing)
