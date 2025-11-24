# Confab Development Notes

## Running Tests

```bash
# CLI
cd cli && go test ./...

# Backend (requires Orbstack/Docker for integration tests)
cd backend && DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...

# Backend unit tests only (skip integration tests)
cd backend && go test -short ./...
```

## Finding Dead Code (Go)

Two complementary tools for detecting unused code in the `backend/` and `cli/` directories:

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
