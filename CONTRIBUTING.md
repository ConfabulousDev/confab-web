# Contributing to Confabulous

Thanks for your interest in contributing! Confabulous is MIT-licensed open source, and contributions — bug reports, feature ideas, docs fixes, and code — are welcome.

## Reporting bugs and requesting features

Open an issue on [GitHub](https://github.com/ConfabulousDev/confab-web/issues). For bugs, include steps to reproduce, what you expected, and what happened. For features, describe the problem you're trying to solve.

## Contributing code

1. Fork the repo and create a branch from `main`.
2. Make your change, with tests where it makes sense.
3. Run the build, lint, and test gates before opening a PR:
   - **Backend:** `go test ./...` (from `backend/`).
   - **Frontend:** `npm run build && npm run lint && npm test` (from `frontend/`).
   - Or both at once from the repo root: `make test`.
4. Open a pull request describing what changed and why.

## Local development

See [Local Development](README.md#local-development) in the README for the full setup. In short:

```bash
make setup    # first run only: creates backend/.env, installs frontend deps
make dev      # infra + backend + frontend in one terminal
```

Then open [http://localhost:5173](http://localhost:5173).

> **Note:** the dev loop runs only Postgres + MinIO in Docker, defined in `docker-compose.infra.yml` (driven by `make up`/`make dev`). The root `docker-compose.yml` is the canonical **self-host deploy** stack — running a bare `docker compose up` at the repo root launches the full app from the prebuilt image, not the dev infra. For local development always use the `make` targets.

## Project guides

Each package documents its own conventions:

- [`backend/README.md`](backend/README.md) and [`backend/CLAUDE.md`](backend/CLAUDE.md)
- [`frontend/README.md`](frontend/README.md) and [`frontend/CLAUDE.md`](frontend/CLAUDE.md)
- [`docs/`](docs/) — the user-facing documentation site

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
