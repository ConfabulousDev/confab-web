# recapquota

Per-user monthly quota tracking for smart recap computations, with admin-facing aggregate statistics.

## Files

| File | Role |
|------|------|
| `recapquota.go` | Quota CRUD operations, per-user stats, and aggregate totals -- all via direct SQL |
| `recapquota_test.go` | Integration tests for month rollover, increment, and `ForMonth` variants |

## Key Types

- **`Quota`** -- A user's quota record: user ID, compute count, quota month (`"YYYY-MM"` format), last compute timestamp, and created-at.
- **`UserStats`** -- Per-user stats for admin display: user ID, email, name, sessions with recap cache, computations this month, last compute timestamp.
- **`Totals`** -- Aggregate totals across all users: non-empty sessions, sessions with cache, computations this month, users with a quota row for the current month.

## Key API

### Quota operations

- **`GetOrCreate(ctx, conn, userID) (*Quota, error)`** -- Retrieves or creates a quota row. If the stored month is stale, atomically resets the count to 0. Uses an `INSERT ... ON CONFLICT DO UPDATE` upsert.
- **`Increment(ctx, conn, userID) error`** -- Bumps the compute count by 1, creating the row if needed and resetting if the month is stale. Sets `last_compute_at` to `NOW()`.
- **`GetCount(ctx, conn, userID) (int, error)`** -- Returns the current month's compute count (0 if no row or stale month).
- **`CurrentMonth() string`** -- Returns the current UTC month as `"YYYY-MM"`.

### Test variants

- **`GetOrCreateForMonth`**, **`IncrementForMonth`**, **`GetCountForMonth`** -- Same as above but accept an explicit month string, enabling deterministic tests.

### Admin statistics

- **`ListUserStats(ctx, conn) ([]UserStats, error)`** -- Joins users, `session_card_smart_recap`, and `smart_recap_quota` to produce per-user recap statistics. Only includes users with activity.
- **`GetTotals(ctx, conn) (*Totals, error)`** -- Three separate queries for aggregate counts: non-empty sessions (sessions in `sync_files` with transcript/agent files having `SUM(last_synced_line) > 0`), sessions with recap cache, and monthly quota totals (total computations and user count).

## How to Extend

### Adding a quota limit check

1. Call `GetOrCreate` or `GetCount` to get the current count.
2. Compare against the configured limit.
3. Call `Increment` only after a successful computation.

### Adding a new aggregate stat

1. Add a field to `Totals`.
2. Add a query in `GetTotals` to populate it.
3. Display it in the admin UI (`internal/admin/handlers.go`).

## Invariants

- **Atomic month rollover.** `GetOrCreate` and `Increment` use `INSERT ... ON CONFLICT DO UPDATE` with a `CASE` expression that resets the count when the stored `quota_month` does not match the target month. This is a single atomic SQL statement -- no read-then-write race.
- **One row per user.** The `smart_recap_quota` table has a unique constraint on `user_id`. The upsert ensures exactly one row per user at all times.
- **UTC month boundaries.** `CurrentMonth()` uses `time.Now().UTC()`, and the SQL queries use `NOW() AT TIME ZONE 'UTC'`. All month comparisons are in UTC.
- **Uses raw `*sql.DB`, not the `db.DB` wrapper.** Functions accept `*sql.DB` (via `conn()` on the DB wrapper) rather than the internal `db.DB` struct, keeping this package decoupled from the DB layer.

## Design Decisions

**Standalone functions instead of a Store struct.** Unlike other `db/` sub-packages that use a `Store` struct, this package exposes free functions that accept `*sql.DB`. The quota table is simple enough that a struct would add ceremony without benefit, and it avoids a circular dependency since `internal/analytics` and `internal/admin` both use it.

**`ForMonth` variants for testing.** Rather than injecting a clock, the time-sensitive functions have `ForMonth` counterparts that accept an explicit month string. This keeps the production API clean while enabling deterministic tests.

**Three separate queries in `GetTotals`.** Each aggregate comes from a different table with different join logic. Combining them into one query would be complex and harder to maintain. The three queries are fast (counting indexed rows) and run within a single request context.

## Testing

```bash
go test ./internal/recapquota/...
```

Integration tests use `testutil.SetupTestEnvironment(t)` with a real Postgres container. Tests cover month rollover, increment idempotency, and the `ForMonth` variants.

## Dependencies

**Uses:** `database/sql` (standard library)

**Used by:** `internal/admin` (admin UI stats), `internal/analytics` (smart recap computation), `internal/api` (quota checks)
