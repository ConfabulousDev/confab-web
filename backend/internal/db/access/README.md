# access

Session access control and share management (create, list, revoke shares; check access type).

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct definition and OpenTelemetry tracer |
| `access.go` | `GetSessionAccessType` (determines how a user can access a session) and `GetSessionDetailWithAccess` (returns session detail with PII redaction for non-owners) |
| `shares.go` | Share CRUD: `CreateShare`, `CreateSystemShare`, `ListShares`, `ListAllUserShares`, `ListSystemShares`, `RevokeShare`, `CountUserSharesSince` (daily-quota counter), `DeleteExpiredShares` (periodic housekeeping), and the private `loadShareRecipients` helper |

## Key API

- **`GetSessionAccessType(ctx, sessionID, viewerUserID)`** -- Determines access level by checking, in order: owner, `ShareAllSessions` flag, then share rows (recipient > system > public). Returns a `SessionAccessInfo` with the access type, share ID, and an `AuthMayHelp` hint for unauthenticated users.
- **`GetSessionDetailWithAccess(ctx, sessionID, viewerUserID, accessInfo)`** -- Loads full session detail for any user with access. Redacts PII (hostname, username, cwd, transcript path) for non-owners. For **public** access (reachable anonymously) it additionally blanks `OwnerEmail` and leaves `SharedByEmail` nil so the owner's email never leaks to anonymous viewers (p99d); recipient/system viewers are authenticated and entitled to it, so they keep both. For **all** non-owners it also sanitizes the free-form `git_info` JSONB via `db.SanitizeGitInfoForSharing` — keeping only `branch` and a host/credential-stripped `owner/repo` display name, dropping remote URLs (which can embed credentials), `tracking_remote`, author, and every other key (d29s). Blocks access if the session owner is deactivated. Updates `last_accessed_at` on the share as a non-critical analytics side effect.
- **`CreateShare(ctx, sessionID, userID, isPublic, expiresAt, recipientEmails)`** -- Creates a public or recipient-only share in a transaction. For recipient shares, batch-resolves email addresses to user IDs.
- **`CreateSystemShare(ctx, sessionID, expiresAt)`** -- Creates a system-wide share (admin operation, no ownership check). Accessible to any authenticated user. **Does not verify admin status itself** — callers must enforce admin auth before invoking (the HTTP handler sits behind `admin.Middleware`).
- **`ListShares(ctx, sessionID, userID)` / `ListAllUserShares(ctx, userID)` / `ListSystemShares(ctx)`** -- Lists shares for a session, across all of a user's sessions, or all system-wide shares (admin). Each filters out expired shares (`expires_at <= NOW()`) so owners/admins never see ghost rows for shares that no longer grant access; the owner lists also include recipient emails (CF-433 / H3).
- **`RevokeShare(ctx, shareID, userID)`** -- Deletes a share, verified through session ownership. Returns `ErrUnauthorized` for both not-found and wrong-owner cases (security by obscurity).
- **`CountUserSharesSince(ctx, userID, since)`** -- Counts shares the user has created since an instant, joining `session_shares` → `sessions` on the owning `user_id` (the table has no owner column). Backs the per-user daily share-creation quota (CF-429 / H2) enforced in the `POST /sessions/{id}/share` handler.
- **`DeleteExpiredShares(ctx, olderThan)`** -- Hard-deletes shares whose `expires_at` is older than `now - olderThan`, returning the count removed. Periodic housekeeping (reclaims storage), **not** a security control — the list/access queries already hide every expired share. A single `DELETE` on `session_shares` suffices: the join tables cascade via `ON DELETE CASCADE`. NULL `expires_at` (never-expiring) rows are excluded. Called from the background worker each cycle, gated by `WORKER_SHARE_RETENTION` (default 30 days / `720h`); see `v9mc` (deferred from 0as2 / CF-433 H3 D1).

## How to Extend

1. **New share type**: Add a new `session_share_*` join table, add a case to the combined access query in `GetSessionAccessType`, and update `CreateShare`/`ListShares` to handle the new type.
2. **New access check dimension**: Add logic between the owner check and the combined share query in `GetSessionAccessType`.
3. **Additional PII redaction fields**: Add to `SessionDetail.RedactForSharing()` in the root `db/types.go`.

## Invariants

- Access check priority: owner > `ShareAllSessions` flag > recipient share > system share > public share.
- Share expiration is enforced in **all** share queries via `(ss.expires_at IS NULL OR ss.expires_at > NOW())` — both the access-grant query in `GetSessionAccessType` and the three list queries (`ListShares`, `ListAllUserShares`, `ListSystemShares`). The list queries must keep this predicate so listings never surface expired ghost rows (CF-433 / H3).
- Deactivated session owners cause `ErrOwnerInactive` for all non-owner access, preventing visibility of content from disabled accounts.
- PII fields are never returned for non-owner access (enforced via `RedactForSharing()`).
- The owner's email is never exposed to **public** (anonymous-reachable) access: `OwnerEmail` is blanked and `SharedByEmail` stays nil for `SessionAccessPublic`. Recipient/system access keeps both — the viewer is authenticated and entitled to know who shared with them (p99d).
- `git_info` is sanitized for **all** non-owner access (recipient, system, and public alike): only `branch` and a host/credential-free `owner/repo` display name survive; remote URLs, host metadata, and committer identity are dropped. This is enforced at read time via `db.SanitizeGitInfoForSharing` after the git_info unmarshal — `RedactForSharing` can't reach it (it runs before the unmarshal and only nils pii-tagged `*string` fields). `TestSessionDetail_InterfaceFieldsAreClassified` guards against a new untagged `interface{}`/JSONB field slipping the same gap (d29s).
- `RevokeShare` uses a single `DELETE ... USING sessions` to atomically verify ownership and delete, preventing TOCTOU races.
- The share tables use a polymorphic pattern: `session_shares` is the base, with `session_share_public`, `session_share_recipients`, and `session_share_system` as type-specific join tables.
- `db.SessionShare.Provider` is always the canonical session provider. Every share-store read (`CreateShare`, `CreateSystemShare`, `ListShares`, `ListSystemShares`, `ListAllUserShares`) selects `session_type` from the joined `sessions` row and applies `models.NormalizeProvider`, so the legacy `"Claude Code"` form is never surfaced to callers (CF-370).

## Design Decisions

- **Single-query access check**: The combined share query uses LEFT JOINs across all share type tables and a CASE expression to determine the highest-priority access in one database round-trip.
- **`AuthMayHelp` flag**: When an unauthenticated user has no public share access but non-public shares exist, this flag tells the frontend to show a login prompt rather than a 404.
- **Non-critical `last_accessed_at` update**: Uses fire-and-forget error handling (`_, _ =`) because failing to record access analytics should never block session viewing.
- **Batch recipient resolution**: `CreateShare` resolves all recipient emails to user IDs in a single query, then batch-inserts all rows, minimizing round-trips.

## Testing

- Integration tests: `access_test.go` (access type resolution, detail retrieval with redaction), `shares_test.go` (share lifecycle, recipients, revocation, and expired-share filtering on all three list endpoints)
- Tests cover all access paths: owner, recipient, system, public, unauthenticated, and deactivated owner scenarios.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for types, errors, helpers
- `github.com/ConfabulousDev/confab-web/internal/models` -- `UserStatus` enum for owner deactivation check
- `go.opentelemetry.io/otel` -- Distributed tracing
