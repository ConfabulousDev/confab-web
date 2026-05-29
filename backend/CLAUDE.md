# Backend Development Notes

## What belongs in this file

Backend conventions Claude would get wrong by default — commands, sync rules, backend-only invariants. For architecture see `README.md`; for the package index see `internal/README.md`; for per-package conventions read the package README. Add to this file only when the rule is (a) backend-wide, (b) non-obvious from reading the code, and (c) Claude would get it wrong without the instruction.

## Running tests

Full tests (final verification — requires Docker/Orbstack):

```bash
go test ./...
```

Unit-only iteration:

```bash
go test -short ./...
```

Sharded (parallel package-at-a-time, what CI runs):

```bash
./scripts/list-test-packages.sh
# then one `go test <pkg>` per line, parallelized
```

Integration tests use `testutil.SetupTestEnvironment(t)` for a containerized Postgres + MinIO.

## When adding a field to a shared DB struct

Adding a column to `db.SessionDetail`, `db.SessionListItem`, or any struct loaded from SQL in more than one place:

1. Update the shared column list (`db.SessionDetailColumns` / `sessionSelectCols`) AND the shared Scan-target helper (`db.SessionDetailScanTargets` / `scanSessionListItems`) — both in one edit, since their order must match.
2. Grep for every other Scan into the struct (`grep -rn 'db.SessionDetail{' .`). Confirm each goes through the shared helpers, not a hand-rolled column list.
3. Add a wire-level assertion in the relevant `*_http_integration_test.go`: read the new field off the JSON response and check its value. Subset-of-columns drift is invisible to type checks; the wire test is the cheapest reliable guard.
4. If the new field has provider/legacy/canonical normalization (like `Provider`), include each variant in the test table.

This rule exists because Go's `database/sql` silently allows Scan over a subset of columns. A parallel column list is one ghost-field bug waiting to happen.

## Updating model pricing

Prices live in **one** place: `internal/pricingsource/pricing.json` (provider-nested, embedded via `go:embed`). To add or reprice a model, edit that file and bump its `updated_at`. There is no second table to keep in sync — the backend cost compute and the frontend both read from it.

The backend serves the effective table at `GET /api/v1/pricing` and refreshes it from confabulous.dev at runtime, so self-hosters pick up new prices without a redeploy (see `internal/pricingsource/README.md`). Read `internal/analytics/README.md` before adding an OpenAI entry — its billing conventions (cached_input as subset, reasoning_output as subset, free cache writes) differ from Anthropic's.

Anthropic prices: https://www.anthropic.com/pricing
OpenAI prices: https://developers.openai.com/api/docs/pricing

## Finding dead code

- `staticcheck ./...` — unused unexported code (functions, types, vars, constants).
- `deadcode -test ./...` — whole-program reachability from `main()` and tests.

Neither catches unused **exported** identifiers.
