# Confab Backend

Backend service for Confab — the self-hosted Claude Code and Codex session analytics platform. See the [root README](../README.md) for deployment instructions.

## Features

- PostgreSQL database for session metadata
- MinIO (S3-compatible) object storage for session files
- RESTful API for chunked session sync
- API key authentication for the CLI; OAuth (GitHub/Google/OIDC) and password auth for the web dashboard
- Multi-user support

## Architecture

```
┌─────────────┐
│  Confab CLI │
└──────┬──────┘
       │ POST /api/v1/sync/{init,chunk,event}
       ▼
┌─────────────┐       ┌──────────────┐
│   Backend   ├──────▶│  PostgreSQL  │
│   (Go)      │       └──────────────┘
└──────┬──────┘
       │
       ▼
┌─────────────┐
│    MinIO    │
│  (S3 API)   │
└─────────────┘
```

## Local Development

The dev loop runs through the root `Makefile` (`make dev`, or `make help` for all
targets) — see **[Local Development](../README.md#local-development)** in the root README.

Backend-specific notes:

- Run the server with `go run ./cmd/server`, not `go run cmd/server/main.go` — the
  `cmd/server` package is multi-file (`main.go` + `worker.go`) and won't compile that way.
- The backend reads all config from the environment. `make dev` / `make backend` load
  `backend/.env` for you; if you invoke `go run` directly, source it first
  (`set -a && . ./.env && set +a`).

### Environment Variables

See [`../CONFIGURATION.md`](../CONFIGURATION.md) for the full environment variable reference. [`.env.example`](.env.example) is a copy-paste-ready template for local development.

## API Endpoints

See [`API.md`](API.md) for the full REST reference. Quick links:

- `GET /health` — liveness probe.
- `POST /api/v1/sync/init`, `POST /api/v1/sync/chunk`, `POST /api/v1/sync/event` — CLI upload flow (chunked).
- `GET /api/v1/sessions`, `GET /api/v1/sessions/{id}` — web dashboard session list and detail.
- `GET /api/v1/sessions/{id}/analytics` — cached card computation plus on-demand smart recap.

## Documentation

- **[SECURITY.md](SECURITY.md)** - Complete security guide (authentication, CORS, CSRF, input validation, headers)
- **[PERFORMANCE.md](PERFORMANCE.md)** - Performance optimization guide (rate limiting, compression, monitoring)
- **[TEST.md](TEST.md)** - Testing guide
- **[TODO.md](TODO.md)** - Future improvements and roadmap

## Database Schema

Schema is managed via [`internal/db/migrations/`](internal/db/migrations/) using `golang-migrate`. See [`DB_MIGRATION_STRATEGY.md`](DB_MIGRATION_STRATEGY.md) for the historical decision context and [`internal/db/README.md`](internal/db/README.md) for the modular DB layer.

## Development

Backend targets (run from `backend/`, or via the root `Makefile`):

```bash
make test       # go test ./...
make coverage   # sharded coverage (see internal/testutil/README.md)
make build      # build the binary
make fmt        # gofmt + go fix
make fmt-check  # CI format gate (gofmt + go fix -diff)
```

## License

MIT
