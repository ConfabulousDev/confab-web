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

### 2. Test Coverage

Every change should include appropriate tests:
- Write tests before or alongside implementation code (TDD encouraged)
- Consider both happy paths and error cases
- Ensure existing tests still pass after changes

### 3. Self-Review Before Presenting

**Before presenting any result to the human, perform a thorough code review:**
- Re-read all code changes critically, as if reviewing someone else's work
- Check for bugs, edge cases, error handling, and security issues
- Verify code follows existing patterns and conventions in the codebase
- Ensure no debug code, TODOs, or incomplete implementations remain
- Run all relevant tests and fix any failures
- Fix any issues found during review before showing the result

This self-review step is mandatory. Do not present work that hasn't been thoroughly reviewed and tested.

## Running Tests

```bash
# Backend (requires Orbstack/Docker for integration tests)
cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...

# Backend unit tests only (skip integration tests)
cd backend && go test -short ./...

# Frontend
cd frontend && npm run build && npm run lint && npm test
```

### Sharded Backend Tests (Faster)

For faster test runs, shard by package using 6 parallel Bash tool calls:

```bash
# Shard 1: db - session/sync tests (~140s, bottleneck)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "Session|Sync" ./internal/db/...

# Shard 2: db - user/oauth tests (~85s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "User|OAuth" ./internal/db/...

# Shard 3: db - auth-related tests (~110s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "APIKey|Device|Share|Web" ./internal/db/...

# Shard 4: api package (~110s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/api/...

# Shard 5: auth package (~40s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/auth/...

# Shard 6: Everything else (~15s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/analytics/... ./internal/storage/... ./internal/admin/... ./internal/clientip/... ./internal/email/... ./internal/ratelimit/... ./internal/validation/...
```

Claude Code can run multiple Bash commands in parallel and aggregate results across all shards.

Note: CLI is in a separate repo: https://github.com/ConfabulousDev/confab-cli

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

## Adding Analytics Cards

When adding new analytics cards to the session summary panel, **use the `/add-session-card` skill**. This provides a step-by-step playbook covering:

- Database migrations (card-per-table architecture)
- Backend collector, types, store operations, and compute logic
- Frontend Zod schemas, components, and registry
- Storybook stories and testing requirements

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
