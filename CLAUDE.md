# Confab Development Notes

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

## Finding Dead Code (Go)

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
