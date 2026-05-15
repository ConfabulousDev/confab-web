# codex

Metadata sidecar for Codex parent-child rollout trees. Child rollouts upload
their chunk content under the root's hosted session as `sync_files` rows with
`file_type='agent'` (CF-389), alongside their per-thread `codex_rollouts`
metadata row. The root's own rollout lands as `file_type='transcript'`. This
package only records the thread tree shape and per-thread metadata.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct and OpenTelemetry tracer |
| `rollouts.go` | `Rollout` type, `UpsertRolloutParams`, `UpsertRollout`, `GetRollout`, `ListSubtree` (recursive CTE) |
| `rollouts_test.go` | Integration tests (Docker-backed) |

## Key API

- **`UpsertRollout(ctx, userID, params)`** — Inserts or updates a `codex_rollouts` row keyed on `(user_id, thread_uuid)`. Idempotent under re-call: first-write-wins on `parent_thread_uuid`; free-form fields (`rollout_path`, `cwd`, `model`, `source`, `thread_source`, `agent_*`) preserve their stored value when the incoming value is empty, and overwrite when the incoming value is non-empty. `updated_at` advances on every successful call.
- **`GetRollout(ctx, userID, threadUUID)`** — Returns one row by `(user_id, thread_uuid)`. Returns `db.ErrRolloutNotFound` when no such row exists.
- **`ListSubtree(ctx, userID, rootThreadUUID)`** — Returns the row matching `rootThreadUUID` plus every descendant reachable through `parent_thread_uuid` edges, ordered by `created_at ASC`. Uses `UNION` (not `UNION ALL`) so any cycle in the input terminates naturally via per-iteration row deduplication. Returns an empty slice when the root UUID does not exist (orphan-parent queries).

## Invariants

- **Composite PK `(user_id, thread_uuid)`** — UUIDs are user-local. Two users may legitimately hold the same `thread_uuid`; they get independent rows.
- **No FK on `parent_thread_uuid`** — Upload order is not guaranteed. A child rollout may be ingested before its parent. Read paths must tolerate missing parents (the recursive CTE just yields nothing for orphan-parent queries).
- **First-write-wins on `parent_thread_uuid`** — Once a row has a non-null parent, later upserts cannot overwrite it (`COALESCE(codex_rollouts.parent_thread_uuid, EXCLUDED.parent_thread_uuid)`). A `nil → set` transition is allowed; a `set → different value` is silently preserved as the original.
- **Cascade deletes** — `ON DELETE CASCADE` from both `users(id)` and `sessions(id)` (via `hosted_session_id`) removes rollout rows when the owner or hosted session is deleted.
- **NULL string columns scan as `""`** — `GetRollout` and `ListSubtree` both apply `COALESCE(<col>, '')` in their SELECT so callers never see Go-zero-vs-NULL ambiguity. Only `parent_thread_uuid` (`*string`) retains nullability.

## How to Extend

1. **New scalar field**: add a `VARCHAR(...)` column in a new migration, append to `UpsertRolloutParams`, the upsert SQL (both the INSERT column list and the `ON CONFLICT DO UPDATE SET` block), and `rolloutColumns` + the `scanRollout` helper. Add a length check in `validation/input.go` and a unit test for the overflow case.
2. **`ListChildren` (direct children only)**: add a non-recursive variant once a frontend endpoint actually needs it. The current `ListSubtree` is sufficient for tree rendering.
3. **Reverse lookup (root from leaf)**: a recursive CTE walking `parent_thread_uuid` upward can be added. Use the same `UNION`-not-`UNION ALL` pattern.

## Wire Path (CF-385)

The `POST /api/v1/sync/chunk` endpoint accepts `metadata.codex_rollout`. The handler:

1. Validates shape and lengths via `validation.ValidateCodexRolloutMetadata` (before session-ownership check, since the validation is stateless).
2. After `VerifySessionOwnership`, rejects the request with 400 if the session's provider is not `codex`.
3. After the S3 chunk upload and `UpdateSyncFileState`, calls `UpsertRollout`. This ordering matches the existing Claude-Code chunk path (blob first, DB second) and accepts the "S3 chunk landed but metadata sidecar delayed" failure mode — the CLI's next chunk's idempotent upsert reconciles.

## Testing

Integration tests live in `rollouts_test.go` and require Docker (Postgres container via `testutil.SetupTestEnvironment`). HTTP-level coverage lives in `internal/api/sync_http_integration_test.go::TestSyncChunk_CodexRollout_HTTP_Integration`.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` — Root DB package (handle, sentinel errors, `NormalizeProvider`)
- `go.opentelemetry.io/otel` — Distributed tracing
