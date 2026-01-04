# Confab Development Notes

## API Documentation

Backend API is documented in `backend/API.md`. **Keep this file up to date** when modifying API endpoints, request/response schemas, or authentication.

## Working on Linear Tickets

When given a Linear ticket to work on, always make a plan first before writing any code. Use the TodoWrite tool to break down the ticket into concrete implementation steps. Once the plan is finalized, update the Linear issue with the plan.

## Test Coverage

New features and bug fixes should come with appropriate test coverage. Write tests before or alongside implementation code.

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
# Shard 1: db - session/sync tests (~155s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "Session|Sync" ./internal/db/...

# Shard 2: db - user/oauth tests (~95s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "User|OAuth" ./internal/db/...

# Shard 3: db - auth-related tests (~95s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test -run "APIKey|Device|Share|Web" ./internal/db/...

# Shard 4: api package (~130s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/api/...

# Shard 5: auth package (~40s)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/auth/...

# Shard 6: Everything else (fast)
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./internal/analytics/... ./internal/storage/... ./internal/admin/... ./internal/clientip/... ./internal/email/... ./internal/ratelimit/... ./internal/validation/...
```

Claude Code can run multiple Bash commands in parallel and aggregate results across all shards.

Note: CLI is in a separate repo: https://github.com/ConfabulousDev/confab-cli

## Frontend Development

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
