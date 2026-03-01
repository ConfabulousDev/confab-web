# Confab Development Notes

## API Documentation

Backend API is documented in `backend/API.md`. **Keep this file up to date** when modifying API endpoints, request/response schemas, or authentication.

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

#### Where Shared Code Lives

- **Chunk operations** (download, merge, parse keys): `internal/storage/chunks.go`
- **Analytics computation**: `internal/analytics/` package
- **Card staleness validation**: `internal/analytics/cards.go` (`IsValid`, `AllValid` methods)
- **Smart recap generation**: `internal/analytics/smart_recap_generator.go`

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

For faster test runs, shard by package using 9 parallel Bash tool calls:

```bash
# Shard 1: db - session tests (~240s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "Session" -skip "AccessType|WithAccess|Sync|WebSession" ./internal/db/...

# Shard 2: db - session access tests (~120s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "AccessType|WithAccess" ./internal/db/...

# Shard 3: db - sync/web session tests (~50s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "Sync|WebSession|ConnectWithRetry|Tsquery" ./internal/db/...

# Shard 4: db - user/oauth/password tests (~85s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "User|OAuth|Password|SmartRecap" ./internal/db/...

# Shard 5: db - auth-related tests (~110s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "APIKey|Device|Share|Web" ./internal/db/...

# Shard 6: api package (~110s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/api/...

# Shard 7: auth package (~40s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/auth/...

# Shard 8: analytics package (~230s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/analytics/...

# Shard 9: remaining packages (~15s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/storage/... ./internal/admin/... ./internal/anthropic/... ./internal/clientip/... ./internal/email/... ./internal/ratelimit/... ./internal/validation/...
```

Claude Code can run multiple Bash commands in parallel and aggregate results across all shards.

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

When adding a new Anthropic model, update the pricing tables in **both** places (they must stay in sync):

- **Backend**: `backend/internal/analytics/pricing.go` — `modelPricingTable`
- **Frontend**: `frontend/src/utils/tokenStats.ts` — `MODEL_PRICING`

Look up current prices on the Anthropic pricing page.

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

Tags follow semver (`v0.3.6`, `v0.3.7`, etc.). To release:

```bash
git checkout main && git pull origin main
git tag v0.X.Y
git push origin v0.X.Y
gh release create v0.X.Y --notes "<release notes>"
```

### Release Notes

Do **not** use `--generate-notes`. Write release notes manually by reviewing all commits since the last tag (`git log <prev-tag>..HEAD --oneline`). Include:

- **All commits listed with descriptions**, grouped by category (Features, Security, Refactoring, Docs, CI, etc.)
- **Link to PRs** where commits have them (e.g., `[#20](url)`)
- **Breaking changes section** covering: renamed/removed env vars, API changes, DB migrations, CLI impact
- Note if there are **no** breaking changes in a given category (no migrations, no CLI changes, etc.)
