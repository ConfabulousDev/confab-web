# dbadminsettings

Key-value store for admin-configurable settings, backed by the `admin_settings` table.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct with `Get`, `Set`, and `Delete` methods |
| `store_test.go` | Integration tests for all store operations |

## Key Types

- **`Setting`** -- Represents a row in `admin_settings`: `Key` (string), `Value` (string), `UpdatedAt` (time.Time).
- **`Store`** -- Holds a `*db.DB` reference with a `conn()` helper (follows the standard db sub-package pattern).

## Key API

- **`Get(ctx, key)`** -- Retrieves a setting by key. Returns `(nil, nil)` when the key does not exist; returns `(&Setting{Value: ""}, nil)` for a row with an empty value (distinct from missing).
- **`Set(ctx, key, value)`** -- Creates or updates a setting atomically via upsert. Sets `updated_at` to `now()`.
- **`Delete(ctx, key)`** -- Removes a setting by key. No error if the key does not exist.

## Current Keys

| Key | Purpose | Written by |
|-----|---------|------------|
| `smart_recap_system_prompt` | Custom instructions for the smart recap LLM prompt | Admin settings API (`PUT /api/v1/admin/settings/smart-recap-prompt`) |
| `smart_recap_regen_requested_at` | RFC 3339 timestamp triggering bulk regeneration | Admin regenerate-all API (`POST /api/v1/admin/settings/smart-recap-prompt/regenerate-all`) |

## Invariants

- The `key` column is the primary key; upsert semantics ensure no duplicates.
- `Get` returns `nil` for missing keys, not an error. Callers must distinguish `nil` (missing) from `&Setting{Value: ""}` (explicitly empty).
- `Delete` is idempotent; deleting a non-existent key is not an error.

## Testing

Integration tests use `testutil.SetupTestEnvironment(t)` with containerized Postgres. Tests cover round-trip get/set/delete behavior and the nil-vs-empty distinction.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for the `DB` handle
