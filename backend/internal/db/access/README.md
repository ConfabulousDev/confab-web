# access

Session access control and share management (create, list, revoke shares; check access type).

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct definition and OpenTelemetry tracer |
| `access.go` | `GetSessionAccessType` (determines how a user can access a session) and `GetSessionDetailWithAccess` (returns session detail with PII redaction for non-owners) |
| `shares.go` | Share CRUD: `CreateShare`, `CreateSystemShare`, `ListShares`, `ListAllUserShares`, `RevokeShare`, and the private `loadShareRecipients` helper |

## Key API

- **`GetSessionAccessType(ctx, sessionID, viewerUserID)`** -- Determines access level by checking, in order: owner, `ShareAllSessions` flag, then share rows (recipient > system > public). Returns a `SessionAccessInfo` with the access type, share ID, and an `AuthMayHelp` hint for unauthenticated users.
- **`GetSessionDetailWithAccess(ctx, sessionID, viewerUserID, accessInfo)`** -- Loads full session detail for any user with access. Redacts PII (hostname, username, cwd, transcript path) for non-owners. Blocks access if the session owner is deactivated. Updates `last_accessed_at` on the share as a non-critical analytics side effect.
- **`CreateShare(ctx, sessionID, userID, isPublic, expiresAt, recipientEmails)`** -- Creates a public or recipient-only share in a transaction. For recipient shares, batch-resolves email addresses to user IDs.
- **`CreateSystemShare(ctx, sessionID, expiresAt)`** -- Creates a system-wide share (admin operation, no ownership check). Accessible to any authenticated user.
- **`ListShares(ctx, sessionID, userID)` / `ListAllUserShares(ctx, userID)`** -- Lists shares for a session or across all of a user's sessions, including recipient emails.
- **`RevokeShare(ctx, shareID, userID)`** -- Deletes a share, verified through session ownership. Returns `ErrUnauthorized` for both not-found and wrong-owner cases (security by obscurity).

## How to Extend

1. **New share type**: Add a new `session_share_*` join table, add a case to the combined access query in `GetSessionAccessType`, and update `CreateShare`/`ListShares` to handle the new type.
2. **New access check dimension**: Add logic between the owner check and the combined share query in `GetSessionAccessType`.
3. **Additional PII redaction fields**: Add to `SessionDetail.RedactForSharing()` in the root `db/types.go`.

## Invariants

- Access check priority: owner > `ShareAllSessions` flag > recipient share > system share > public share.
- Share expiration is enforced in all queries via `(sh.expires_at IS NULL OR sh.expires_at > NOW())`.
- Deactivated session owners cause `ErrOwnerInactive` for all non-owner access, preventing visibility of content from disabled accounts.
- PII fields are never returned for non-owner access (enforced via `RedactForSharing()`).
- `RevokeShare` uses a single `DELETE ... USING sessions` to atomically verify ownership and delete, preventing TOCTOU races.
- The share tables use a polymorphic pattern: `session_shares` is the base, with `session_share_public`, `session_share_recipients`, and `session_share_system` as type-specific join tables.

## Design Decisions

- **Single-query access check**: The combined share query uses LEFT JOINs across all share type tables and a CASE expression to determine the highest-priority access in one database round-trip.
- **`AuthMayHelp` flag**: When an unauthenticated user has no public share access but non-public shares exist, this flag tells the frontend to show a login prompt rather than a 404.
- **Non-critical `last_accessed_at` update**: Uses fire-and-forget error handling (`_, _ =`) because failing to record access analytics should never block session viewing.
- **Batch recipient resolution**: `CreateShare` resolves all recipient emails to user IDs in a single query, then batch-inserts all rows, minimizing round-trips.

## Testing

- Integration tests: `access_test.go` (access type resolution, detail retrieval with redaction), `shares_test.go` (share lifecycle, recipients, revocation)
- Tests cover all access paths: owner, recipient, system, public, unauthenticated, and deactivated owner scenarios.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for types, errors, helpers
- `github.com/ConfabulousDev/confab-web/internal/models` -- `UserStatus` enum for owner deactivation check
- `go.opentelemetry.io/otel` -- Distributed tracing
