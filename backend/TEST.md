# Backend Testing Guide

## Prerequisites

- Docker / Orbstack (integration tests run Postgres + MinIO in containers)
- Go 1.26+

## Running Tests

Run from the `backend/` directory.

### Unit tests (fast — no Docker)

```bash
go test -short ./...
```

### Full test suite (unit + integration, requires Docker)

```bash
DOCKER_HOST=unix:///Users/santaclaude/.orbstack/run/docker.sock go test ./...
```

The `DOCKER_HOST` path matches the Orbstack default on macOS. Adjust as needed for your container runtime.

### Sharded test runs (faster)

CI shards by package using [`scripts/list-test-packages.sh`](scripts/list-test-packages.sh). Locally:

```bash
./scripts/list-test-packages.sh
# emits one Go package per line; run each in parallel with `go test <package>`
```

See `CLAUDE.md` (in this directory) for the test commands and `scripts/list-test-packages.sh` itself for what packages are picked up.

## Test Patterns

- **Unit tests** (`*_test.go`) cover pure logic (parsers, validators, formatters, pricing).
- **Integration tests** (`*_integration_test.go`, `*_http_integration_test.go`) spin up containerized Postgres and MinIO via `testutil.SetupTestEnvironment(t)`.
- Helpers in [`internal/testutil/`](internal/testutil/) provide common fixtures — see that package's `README.md`.

## Manual End-to-End Smoke Test

To exercise the CLI ⇄ backend sync path against a local stack:

```bash
# 1. Start the local dev stack: `make dev` (see Local Development in the root README).

# 2. Create an API key via the web UI at http://localhost:5173 (or POST /api/v1/keys
#    with an authenticated web session). The bootstrap admin is whatever you set in
#    backend/.env — admin@local.dev / localdevpassword by default.

# 3. Configure the Confab CLI (separate repo: https://github.com/ConfabulousDev/confab)
#    to point at the backend API at http://localhost:8080 with the API key from step 2.

# 4. Run a Claude Code or Codex session. The CLI uploads chunks via /api/v1/sync/{init,chunk,event}.

# 5. Verify in the web UI or directly in Postgres:
docker compose -f docker-compose.infra.yml exec postgres psql -U confab -d confab \
  -c "SELECT external_id, session_type, total_lines FROM sessions ORDER BY created_at DESC LIMIT 5;"
```

## Coverage

```bash
make coverage
```

Runs the full suite one package at a time via [`scripts/coverage.sh`](scripts/coverage.sh) with `-coverpkg=./internal/...`, writes the merged profile to `coverage.out`, and ends with a per-package coverage table. To print that table for an existing profile without running tests: `COVERAGE_SUMMARY_ONLY=coverage.out ./scripts/coverage.sh`.

Use this (or your own `-coverpkg` run) rather than `go test -cover ./...`. Plain `-cover` credits a package only with its own tests, so it under-reports packages exercised from other packages — e.g. `internal/api`, whose HTTP integration tests live in subpackages.
