# dbadmincardinvalidations

DB operations for the `admin_card_invalidations` table. Backs the CF-343 admin
"invalidate cards by date range" feature. The table doubles as an audit log and
as a smart-recap quota-bypass signal.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct with `CountAffected`, `Execute`, `ListRecent`, `ListByCorrelationID` |
| `store_test.go` | Integration tests (intersection semantic, chunked execute, history listing) |

## Key Types

- **`Store`** — Holds a `*db.DB` reference and an optional `BatchSize` override (tests only).
- **`CountRequest`** — `{ StartDate, EndDate *time.Time, CardTypes []string }`.
- **`CountResult`** — `{ AffectedSessions int, AffectedCards map[string]int }`.
- **`ExecuteRequest`** — Embeds `CountRequest`, adds `AdminUserID` and `Reason`.
- **`ExecuteResult`** — `{ CorrelationID uuid.UUID, Result CountResult, CompletedBatches int, Err error }`.
- **`AuditRow`** — Single audit row with `AdminEmail` LEFT JOIN'd from `users` (empty when admin deleted).

## Key API

- **`CountAffected(ctx, CountRequest)`** — Dry-run. Returns distinct session count and per-table row counts with no writes. Intersection semantic: a session is counted only if it has at least one row in one of the selected card tables.
- **`Execute(ctx, ExecuteRequest)`** — Runs the invalidation in `DefaultBatchSize`-session chunks (1000 by default). Each batch DELETEs from selected tables and INSERTs audit rows in a single transaction that commits independently. Stops at the first failure and returns partial progress in `ExecuteResult`. The CorrelationID is generated once and shared across all batches in a run.
- **`ListRecent(ctx, limit)`** — Returns up to `limit` audit rows ordered by `invalidated_at DESC` (`limit <= 0` means 500).
- **`ListByCorrelationID(ctx, uuid.UUID)`** — Returns all audit rows for a single correlation_id, ordered by id.

## Invariants

- **Card types are validated against `analytics.AllCardTableNames`** before any SQL interpolation. Table names are the only part of the query not using parameter binding; the allowlist prevents injection.
- **`admin_user_id` has no FK.** The audit row survives the admin user's deletion; `admin_email` reads as NULL / empty in that case.
- **`card_types` stores the admin's requested selection**, identical for every row in one correlation_id. Simple, sufficient for both audit and smart-recap bypass (which filters on `'session_card_smart_recap' = ANY(card_types)`).
- **Reason is required and non-empty** at both the store layer and via the DB CHECK constraint.
- **Partial-failure invariant.** Already-committed batches remain invalidated; re-running the same window is idempotent at the row level (DELETE of already-deleted rows is a no-op; new audit rows are appended).

## Testing

Integration tests use `testutil.SetupTestEnvironment(t)` with containerized Postgres. Tests set `BatchSize: 2` on the store to exercise chunking without seeding thousands of rows.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/analytics` — `AllCardTableNames` / `IsKnownCardTableName`
- `github.com/ConfabulousDev/confab-web/internal/db` — Root DB handle
- `github.com/google/uuid` — Correlation IDs
- `github.com/lib/pq` — `pq.Array` for `TEXT[]` and `UUID[]` parameters
