# Backend Testing Guide

## Prerequisites

- Docker / Orbstack (integration tests run Postgres + MinIO in containers)
- Go 1.21+

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

See `CLAUDE.md` for the rationale and sharding rules.

## Test Patterns

- **Unit tests** (`*_test.go`) cover pure logic (parsers, validators, formatters, pricing).
- **Integration tests** (`*_integration_test.go`, `*_http_integration_test.go`) spin up containerized Postgres and MinIO via `testutil.SetupTestEnvironment(t)`.
- Helpers in [`internal/testutil/`](internal/testutil/) provide common fixtures — see that package's `README.md`.

## Manual End-to-End Smoke Test

To exercise the CLI ⇄ backend sync path against a local stack:

```bash
# 1. Start the stack
docker compose up -d

# 2. Create an API key (via the web UI at http://localhost:8080, or by hitting POST /api/v1/keys
#    with an authenticated web session). The seeded admin account is admin@local.dev / localdevpassword.

# 3. Configure the Confab CLI (separate repo: https://github.com/ConfabulousDev/confab)
#    to point at http://localhost:8080 with the API key from step 2.

# 4. Run a Claude Code or Codex session. The CLI uploads chunks via /api/v1/sync/{init,chunk,event}.

# 5. Verify in the web UI or directly in Postgres:
docker exec -it confab-postgres psql -U confab -d confab \
  -c "SELECT external_id, session_type, total_lines FROM sessions ORDER BY created_at DESC LIMIT 5;"
```

## Coverage

```bash
make coverage
```

Runs sharded coverage via [`scripts/coverage.sh`](scripts/coverage.sh).
