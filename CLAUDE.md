# Confab Development Notes

## API Documentation

Backend API is documented in `backend/API.md`. **Keep this file up to date** when modifying API endpoints, request/response schemas, or authentication.

## Documentation Maintenance

When changing code, update the corresponding package/module README. Key things to keep current: file lists, exported API descriptions, invariants, dependency lists, and extension checklists. If a change spans multiple packages, also check the index READMEs (`backend/internal/README.md`, `frontend/src/README.md`) and `CLAUDE.md`. Documentation that contradicts the code is worse than no documentation.

## Development Process

**Follow this workflow for all implementation tasks:**

### 1. Plan First

Before writing any code, create a clear plan:
- Use the TodoWrite tool to break down the task into concrete implementation steps
- Consider edge cases, error handling, and potential impacts on existing code
- For Linear tickets, update the issue with the plan before starting implementation

### 2. Avoid Code Duplication (DRY)

**Before implementing any logic, check if similar logic already exists elsewhere.**

#### Common Duplication Patterns to Avoid

1. **Query logic duplicated in SQL and Go** - If you have staleness/validity checks, they should exist in ONE place. Either SQL calls a shared function, or Go is the single source of truth.

2. **Utility functions copied between packages** - Functions like `extractAgentID`, `mergeChunks`, `parseChunkKey` should live in ONE shared location and be imported everywhere.

3. **Business logic in multiple code paths** - If both "on-demand" and "background worker" paths do the same thing, extract the shared logic to a common function.

4. **Hand-rolled SELECT column lists for shared structs** - If two functions Scan into the same DB struct (`db.SessionDetail`, `db.SessionListItem`, …), do **not** repeat the column list in each query. Use the shared `db.SessionDetailColumns` constant + `db.SessionDetailScanTargets` helper (and follow the same pattern for new shared types). Go's `database/sql` lets you Scan a subset of columns silently — every parallel column list is one CF-347-style ghost field waiting to happen.

#### When Adding a Field to a Shared DB Struct

Adding a column to `db.SessionDetail`, `db.SessionListItem`, or any struct loaded from SQL in more than one place:

1. Update the shared column list (`db.SessionDetailColumns` / `sessionSelectCols`) and the shared Scan-target helper (`db.SessionDetailScanTargets` / `scanSessionListItems`) — both in one edit, since their order must match.
2. Grep for every other Scan into the struct: `grep -rn 'db.SessionDetail{' backend/` etc. Audit each to confirm it goes through the shared helpers, not a hand-rolled column list.
3. Add a wire-level assertion: extend the relevant `*_http_integration_test.go` to read the new field off the JSON response and check its value. Subset-of-columns drift is invisible to type checks; the wire test is the cheapest reliable guard.
4. If the new field has provider/legacy/canonical normalization (like `Provider`), include each variant in the test table.

#### Where Shared Code Lives

- **Chunk operations** (download, merge, parse keys): `internal/storage/chunks.go`
- **Analytics computation**: `internal/analytics/` package
- **Card staleness validation**: `internal/analytics/cards.go` (`IsValid`, `AllValid` methods)
- **Smart recap generation**: `internal/analytics/smart_recap_generator.go`
- **SessionDetail SQL projection**: `internal/db/session_detail.go` (`SessionDetailColumns`, `SessionDetailScanTargets`) — shared between `db/session.GetSessionDetail` and `db/access.GetSessionDetailWithAccess`
- **Provider identity & legacy aliasing**: `internal/models/provider.go` (`ProviderClaudeCode`, `ProviderCodex`, `ProviderClaudeCodeLegacy`, `CanonicalProviders`, `AllowedProviders`, `LegacyAliases`, `NormalizeProvider`, `ExpandWithAliases`). This is the **permanent** aliasing layer — Confab is OSS self-hosted, so legacy `session_type` values are never backfilled away. Add new aliases here rather than scattering dual-value handling across the codebase. Every SQL filter that wants "all provider rows" passes `pq.Array(models.AllowedProviders)` to `session_type = ANY($N)`; `TestRegistryCoversAllowedProviders` (in `internal/analytics/`) guards that every allowed DB value resolves through the analytics provider registry.
- **Codex rollout parser**: `internal/codex/parser.go` (`ParseRollout`) — produces a `*ParsedRollout` consumed by analytics
- **Codex → analytics adapter**: `internal/analytics/codex_adapter.go` (`ComputeFromCodexRollout`) — maps a Codex rollout onto the same `ComputeResult` shape as the Claude path. Per-card mapping decisions documented inline (token cache subset, FilesRead=0, etc.)
- **Provider-aware precompute dispatch**: `internal/analytics/provider.go` (`SessionProvider`, `RegisterProvider`, `ProviderFor`) plus `claude_provider.go` and `codex_provider.go`. `PrecomputeRegularCards` / `BuildSearchIndexOnly` / `PrecomputeSmartRecapOnly` do one `ProviderFor(StaleSession.Provider)` lookup and then call the provider interface. Unsupported providers return a loud error; Claude registers both `claude-code` and legacy `Claude Code`.
- **Codex rollout sidecar store**: `internal/db/codex/` (`UpsertRollout`, `GetRollout`, `ListSubtree` recursive CTE) — records the Codex parent-child thread tree without modifying the `sessions` table. Composite PK `(user_id, thread_uuid)`; no FK on `parent_thread_uuid` (orphan parents allowed); first-write-wins on parent.

#### Before Writing New Code

1. Search for existing implementations: `grep -r "functionName" backend/`
2. Check if a similar pattern exists in related code paths
3. If you find duplication, refactor to a shared location FIRST
4. Add comments noting where the shared logic lives (e.g., "This mirrors AllValid() in cards.go")

### 3. Test Coverage

Every change should include appropriate tests. **Insufficient test coverage is not acceptable.**

#### What to Test

1. **Unit tests** for pure logic and helper functions:
   - Data transformation functions
   - Validation logic
   - Parsing/formatting utilities
   - Business rule calculations

2. **Integration tests** for database operations:
   - SQL queries (especially complex ones with JOINs, CTEs, aggregations)
   - CRUD operations and edge cases
   - Constraint violations and error handling
   - Use `testutil.SetupTestEnvironment(t)` for containerized Postgres/MinIO

3. **API tests** for HTTP handlers:
   - Success paths with valid input
   - Error responses for invalid input
   - Authentication/authorization checks
   - Edge cases (empty results, pagination bounds)

#### Test Coverage Checklist

Before presenting work, verify you have tests for:

- [ ] **Happy path**: Does the feature work correctly with valid input?
- [ ] **Edge cases**: Empty inputs, boundary values, nil/null handling
- [ ] **Error cases**: Invalid input, missing data, permission denied
- [ ] **SQL queries**: If you wrote SQL, test it with real data (integration test)
- [ ] **Configuration**: If you added config options, test parsing and validation

#### Test Patterns in This Codebase

```go
// Unit test (runs with -short)
func TestHelperFunction(t *testing.T) {
    // Test pure logic
}

// Integration test (requires Docker, skipped with -short)
func TestDatabaseOperation(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    env := testutil.SetupTestEnvironment(t)
    env.CleanDB(t)
    // Test with real database
}
```

#### When to Ask About Test Coverage

If implementing a feature without tests, pause and ask:
- "What test cases would give us confidence this works?"
- "Are there edge cases I should test?"
- "Should this have integration tests for the SQL queries?"

### 4. Self-Review Before Presenting

**Before presenting any result to the human, perform a thorough code review:**

1. **Re-read all modified files directly** - Use the Read tool to review each changed file. Do not rely solely on memory or tests passing. Actually read the code again with fresh eyes.

2. **Review critically, as if reviewing someone else's work:**
   - Check for bugs, edge cases, and error handling gaps
   - Look for logic errors and off-by-one mistakes
   - Verify interactions between modified components work correctly
   - Check that conditional logic handles all cases (especially error/null states)

3. **Verify code quality:**
   - Follows existing patterns and conventions in the codebase
   - No debug code, TODOs, or incomplete implementations remain
   - No security vulnerabilities introduced

4. **Run all relevant tests and fix any failures**

5. **Fix any issues found during review before showing the result**

This self-review step is mandatory. Tests passing is necessary but not sufficient - bugs can exist in untested code paths. Direct code review catches issues that tests miss.

## Running Tests

**IMPORTANT:** Always run full backend tests (including integration tests) as the final verification step before presenting work. The `-short` flag is only for quick iteration during development - it does NOT provide adequate test coverage.

```bash
# Backend - FULL TESTS (required for final verification)
# Requires Orbstack/Docker for integration tests
cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...

# Backend - UNIT TESTS ONLY (quick iteration during development ONLY)
# NOT sufficient as final verification - use full tests before presenting work
cd backend && go test -short ./...

# Frontend
cd frontend && npm run build && npm run lint && npm test
```

**Important:** Always run frontend commands from the `frontend/` directory using `npm run`.
Do NOT run `tsc`, `eslint`, or `vitest` directly — they are local binaries
resolved via `node_modules/.bin` which `npm run` adds to PATH automatically.
If commands fail with "command not found", run `npm install` first.

### Sharded Backend Tests (Faster)

Use `scripts/list-test-packages.sh` to discover all testable packages, then run one parallel Bash tool call per package:

```bash
# List all packages with tests (one per line):
cd backend && ./scripts/list-test-packages.sh

# Run each package as a separate parallel Bash call:
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test <package>
```

**How to shard:** Run `./scripts/list-test-packages.sh`, then launch one parallel Bash tool call per package with `DOCKER_HOST=... go test <package>`. This is the same discovery CI uses.

**Sharding rule:** Always shard by package, never by test name (`-run`/`-skip`).
When a package is too slow, split it into sub-packages rather than adding name-based filters.
CI discovers testable packages dynamically — no config changes needed when adding packages.

Note: CLI is in a separate repo: https://github.com/ConfabulousDev/confab

## Frontend Development

### Theme Support

The frontend supports light and dark themes. When adding CSS, use theme-aware CSS variables from `frontend/src/styles/variables.css`:

- `--color-bg-primary`, `--color-bg-secondary` for backgrounds
- `--color-text-primary`, `--color-text-secondary`, `--color-text-muted` for text
- `--color-accent`, `--color-accent-hover` for accent colors
- `--color-border` for borders

Avoid hardcoded colors. Test changes in both themes.

### Build and Test

Always run build, lint, and test after every change:

```bash
cd frontend && npm run build && npm run lint && npm test
```

- **Build**: TypeScript compilation + Vite build. Catches type errors.
- **Lint**: ESLint with strict rules. Must have 0 errors (warnings are OK).
- **Test**: Vitest unit tests. All tests must pass.

### Storybook

When adding or modifying frontend components, always add or update Storybook stories:

```bash
cd frontend && npm run build-storybook  # Verify stories build
cd frontend && npm run storybook        # Run locally to preview
```

Stories live alongside components (e.g., `Component.stories.tsx` next to `Component.tsx`).

**All new or modified frontend components must have corresponding Storybook stories.** This ensures visual regression coverage is maintained alongside unit tests. When reviewing PRs, verify that stories exist for any new UI components or significant visual changes.

## Adding Analytics Cards

When adding new analytics cards to the session summary panel, **use the `/add-session-card` skill**. This provides a step-by-step playbook covering:

- Database migrations (card-per-table architecture)
- Backend collector, types, store operations, and compute logic
- Frontend Zod schemas, components, and registry
- Storybook stories and testing requirements

## Updating Model Pricing

When adding a new Anthropic OR OpenAI model, update the pricing tables in **both** places (they must stay in sync; `TestPricingTableSync` enforces this):

- **Backend**: `backend/internal/analytics/pricing.go` — `modelPricingTable`
- **Frontend**: `frontend/src/utils/tokenStats.ts` — `MODEL_PRICING`

Anthropic prices: https://www.anthropic.com/pricing
OpenAI prices: https://developers.openai.com/api/docs/pricing

OpenAI conventions differ from Anthropic:
- OpenAI's `cached_input_tokens` is a **subset** of `input_tokens` (not a separate count). The Codex adapter subtracts it before applying the uncached rate.
- OpenAI does NOT charge for cache writes — set `CacheWrite: decimal.NewFromFloat(0)` on every OpenAI entry.
- OpenAI cache reads are typically ~10% of input (gpt-5 family) — different from Anthropic's 0.1x ratio that uses separate `CacheRead`/`CacheWrite` rates.
- OpenAI model family keys are passed through unchanged (e.g. `"gpt-5"`, `"gpt-5.5"`, `"o3-mini"`); `stripOpenAIDateSuffix` handles pinned date suffixes like `gpt-5-2026-05-01`.

## Finding Dead Code

### Frontend (TypeScript)

Use **knip** to find unused files, exports, and dependencies:

```bash
cd frontend && npm run knip
```

Knip categories:
- **Unused files**: Truly dead code - delete these
- **Unused exports**: Often intentional (barrel files, public API) - use judgment
- **Unused dependencies**: Verify before removing (@types/* may be implicit)

### Backend (Go)

Two complementary tools for detecting unused code in the `backend/` directory:

### staticcheck

Catches unused unexported code (functions, types, vars, constants). Conservative with few false positives.

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
```

### deadcode -test

Whole-program reachability analysis from `main()` and test entry points. Catches dead call chains.

```bash
go install golang.org/x/tools/cmd/deadcode@latest
deadcode -test ./...
```

Note: Neither tool catches unused *exported* identifiers, since those could theoretically be used by external packages.

## Cutting a Release

Use the `/cut-release` skill. See `.claude/skills/cut-release/SKILL.md` for the full process.
