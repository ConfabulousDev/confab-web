# cursor

Per-session metadata sidecar for Cursor sessions. Cursor's synced transcript
JSONL carries no model field (and `meta.json` has none either), so the model
name the CLI recovers from Cursor's app-support `state.vscdb` and sends as
`metadata.model` on sync chunks has no home on the generic `sessions` table.
This package persists it into the `cursor_session_meta` table so the analytics
step can read it back and populate the session card's `models_used` (zsr6). It
mirrors the `db/codex` sidecar shape.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct and OpenTelemetry tracer |
| `meta.go` | `UpsertModel`, `GetModel` |
| `meta_test.go` | Integration tests (Docker-backed) |

## Key API

- **`UpsertModel(ctx, sessionID, model)`** — Inserts or updates the `cursor_session_meta` row keyed on `session_id`. **First non-empty model wins**: a re-upsert with a different model never clobbers an already-stored value (`COALESCE(NULLIF(cursor_session_meta.model, ''), EXCLUDED.model)`). `updated_at` advances on every successful call. Callers pass only a non-empty model (the sync handler guards on this).
- **`GetModel(ctx, sessionID)`** — Returns `(model, found, err)`. `found == false` (with `model == ""`) means no row exists yet; the caller leaves `models_used` empty rather than inventing a model.

## Invariants

- **PK `session_id`** — One row per session; FK to `sessions(id)` with `ON DELETE CASCADE`, so the row disappears with its session.
- **`model` is `NOT NULL`** — The app only ever inserts a non-empty model (empty/absent `metadata.model` skips the upsert entirely), per the project convention that required values are supplied by the app, not defaulted/constrained in the DB.
- **First-non-empty-wins** — Because `model` is never stored empty, `COALESCE(NULLIF(existing, ''), excluded)` always preserves the first stored value; later chunks never overwrite it.

## How to Extend

1. **New scalar field**: add a column in a new migration, extend `UpsertModel` (or add a params struct, mirroring `db/codex` once there is more than one field), the upsert SQL, and `GetModel`/a scan helper. Add a length check in `validation/input.go` and a unit test for the overflow case.

## Wire Path (zsr6)

`POST /api/v1/sync/chunk` accepts `metadata.model`. On a cursor transcript chunk
with a non-empty model the handler calls `UpsertModel` after the S3 chunk and
sync-state writes commit (so a failure here leaves only a delayed registration
the next chunk reconciles). The analytics provider (`cursorProvider.Parse`)
reads the model back directly from `cursor_session_meta` and threads it into the
session card's `models_used`.
