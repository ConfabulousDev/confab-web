# Confab Development Notes

## Running Tests

```bash
# Backend (requires Orbstack/Docker for integration tests)
cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...

# Backend unit tests only (skip integration tests)
cd backend && go test -short ./...

# Frontend
cd frontend && npm run build && npm run lint && npm test
```

Note: CLI is in a separate repo: https://github.com/santaclaude2025/confab-cli

## Frontend Development

Always run build, lint, and test after every change:

```bash
cd frontend && npm run build && npm run lint && npm test
```

- **Build**: TypeScript compilation + Vite build. Catches type errors.
- **Lint**: ESLint with strict rules. Must have 0 errors (warnings are OK).
- **Test**: Vitest unit tests. All tests must pass.

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
