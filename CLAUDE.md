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
- **Codex / Claude orchestrators**: `internal/analytics/codex_compute.go` (`ComputeFromCodexRollout`) and `internal/analytics/claude_compute.go` (`ComputeStreaming`) are the per-provider orchestrators only. The shared `ComputeResult` aggregate lives in `internal/analytics/compute_result.go`. Per-card compute lives in `analyzer_<card>_<provider>.go` files (e.g. `analyzer_tokens_codex.go`, `analyzer_conversation_claude.go`). The (card, provider) matrix is scannable via `ls backend/internal/analytics/analyzer_*.go` and `ls backend/internal/analytics/*_compute*.go` (CF-454). Per-card mapping decisions for Codex are documented in `ComputeFromCodexRollout`'s docstring; semantic divergences from Claude (e.g. Conversation card's reasoning-as-active-time synthetic event) live inline in the per-card file. Follow the convention when adding a new card or provider — see `internal/analytics/README.md` for the matrix table. **Subagents & skills (CF-443)**: Codex `spawn_agent` / `wait_agent` function_calls are routed out of `Turn.ToolCalls` by the Codex parser into `ParsedRollout.SubagentSpawns` (so they don't pollute the Tools card); `<skill>` user-message wrappers populate `ParsedRollout.SkillInvocations` and the `<skills_instructions>` developer block populates `ParsedRollout.AvailableSkills`. `computeCodexAgentsAndSkills` buckets spawns by `agent_role` (success iff `wait_agent` reported `"completed"`) and skills by name (always success — Codex emits no per-skill error signal in rollout JSONL).
- **Provider-aware analytics dispatch**: `internal/analytics/provider.go` (`SessionProvider`, `RegisterProvider`, `ProviderFor`) plus `claude_provider.go` and `codex_provider.go`. Both the precompute worker (`PrecomputeRegularCards` / `BuildSearchIndexOnly` / `PrecomputeSmartRecapOnly`) and the on-demand HTTP handler (`api/analytics.go::HandleGetSessionAnalytics`, `HandleRegenerateSmartRecap`) do one `ProviderFor(provider)` lookup and then call the `SessionProvider` interface — no `switch session.Provider` anywhere (CF-402 + CF-403). `SessionProvider.DisplayName()` is also the source of truth for share-email subject lines via `email/email.go::humanProviderLabel`. Both `claudeRollout` and `codexRollout` lazy-materialize multi-file content on first traversal so subsequent methods reuse the cache. Codex sessions aggregate main + subagent rollouts (`sync_files.file_type IN ('transcript','agent')`) across all 7 codex analyzers; the Conversation card stays main-only by design. HTTP intake metadata stays **typed per-provider** (`SyncChunkMetadata.CodexRollout`, validated by `validation.ValidateCodexRolloutMetadata`, persisted via `dbcodex.Store.UpsertRollout`) — the registry is not used at the wire layer. See `backend/internal/analytics/PROVIDER_EXTENSION.md` for the "add a third provider" checklist.
- **Codex rollout sidecar store**: `internal/db/codex/` (`UpsertRollout`, `GetRollout`, `ListSubtree` recursive CTE) — records the Codex parent-child thread tree without modifying the `sessions` table. Composite PK `(user_id, thread_uuid)`; no FK on `parent_thread_uuid` (orphan parents allowed); first-write-wins on parent.
- **Frontend provider abstraction (cosmetic)**: `frontend/src/utils/providers.ts` — `PROVIDER_VALUES`, `PROVIDER_METADATA`, `getProviderMetadata*`, `providerLabel`. Single source of truth for per-provider labels, icons, brand colors, and copy-id menu strings. Mirrors the backend's `models.NormalizeProvider` (CF-416).
- **Frontend provider abstraction (transcript)**: `frontend/src/providers/` — `ProviderAdapter` interface + `getAdapter()` registry + shared `useTranscriptData` / `useSessionTILs` hooks. `SessionViewer` and `SessionHeader` are provider-agnostic and dispatch through the adapter. Adding a third provider's transcript pipeline means writing one adapter file and registering it; no edits to `SessionViewer.tsx` / `SessionHeader.tsx` (CF-417). See `frontend/src/providers/README.md` for the third-provider checklist.
- **Frontend provider abstraction (cost)**: `frontend/src/utils/tokenStats.ts` — canonical `TokenUsage` (`input`/`output`/`cacheWrite`/`cacheRead`) stamped by both transcript services at parse time. Provider-keyed `MODEL_PRICING` (`Record<ProviderId, Record<family, ModelPricing>>`) plus `calculateCost(provider, model, usage)` base arithmetic. Provider-specific cost adjustments (Claude fast multiplier, web-search dollars; Codex tooltip extras) live on `ProviderAdapter.calculateMessageCost` / `extendCostTooltip` in `providers/` (CF-418).
- **Frontend test fixtures (per-provider defaults)**: `frontend/src/test-fixtures/session.ts` — `makeSessionFixture(provider, overrides)` / `makeSessionDetailFixture(provider, overrides)`. `DEFAULTS_BY_PROVIDER` centralizes per-provider default values (external-id prefix, transcript file name) so a third provider extends the fixture coverage by adding one entry rather than touching every story (CF-420).
- **Repo filter fork→root collapsing (CF-491)**: `backend/internal/db/repo_filter.go` (`RepoRootExpr` for SELECT projections, `RepoMatchExpr` for WHERE clauses) is the single source of truth for the regex + `COALESCE(session_repos.root_name, extracted)` pattern used by every repo-filter call site (Sessions filter list/match, TILs filter list/match, `api/org_repos.go`, `analytics/org_analytics.go`, `analytics/trends.go`). The fork→root mapping is written by the resolver in `api/sync.go::HandleSyncChunk` via `db.RecordRepoRoot` (first-write-wins on `session_repos.root_name IS NULL`). When adding a new repo-filter consumer, use the helpers instead of copy-pasting the regex.
- **Demo identity (CF-483)**: `backend/internal/auth/demo.go` — `BootstrapDemoIdentity`, `AutoImpersonateIfDemo`, `EnforceReadOnly` (chained inside every auth middleware, never mounted standalone), `DemoSessionCookieID` (HMAC-derived shared cookie ID), `RenderDemoBannerScriptTag` (XSS-safe `window.__DEMO_IDENTITY__` injection via `html/template.JSEscapeString`), `IsDemoLoginEmail`, `redirectDemoLoginRejected`, `WithReadOnly` / `ReadOnlyFromContext`. Frontend: `frontend/src/utils/demoIdentity.ts` + `components/DemoBanner.tsx` + `components/ReadOnlyToast.tsx`. **Single env var** `DEMO_IDENTITY_EMAIL` activates the whole feature; when unset every demo-mode predicate short-circuits to today's behavior. See `CONFIGURATION.md` and the security review notes in the `auth/README.md` invariants for B1/B2/D1/D2 fixes.

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
- **Frontend**: `frontend/src/utils/tokenStats.ts` — `MODEL_PRICING['claude-code']` or `MODEL_PRICING['codex']` (provider-keyed nested record since CF-418)

Anthropic prices: https://www.anthropic.com/pricing
OpenAI prices: https://developers.openai.com/api/docs/pricing

OpenAI conventions differ from Anthropic:
- OpenAI's `cached_input_tokens` is a **subset** of `input_tokens` (not a separate count). The Codex adapter subtracts it before applying the uncached rate.
- OpenAI's `reasoning_output_tokens` is a **subset** of `output_tokens` (not a separate count, CF-471). Never add it to output or to a total; it bills at the output rate implicitly. Preserve it for display only.
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
