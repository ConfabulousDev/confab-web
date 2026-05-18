# cmd/server

Entry point for the Confab backend. The same binary serves two roles depending on `os.Args[1]`:

| Invocation | Role | What runs |
|---|---|---|
| `server` | HTTP API | `main()` in `main.go` — wires up DB, S3, email, auth, and serves the API on `PORT`. |
| `server worker` | Background precompute worker | `runWorker()` in `worker.go` — polls for stale sessions and recomputes analytics. |

```bash
# API server
go run ./cmd/server

# Background worker
go run ./cmd/server worker
```

This package is intentionally thin. Request handlers live in [`internal/api`](../../internal/api), and precompute logic lives in [`internal/analytics`](../../internal/analytics).

## Server env vars

### Auth — at least one method must be enabled (otherwise the server refuses to start)
| Var | Purpose |
|---|---|
| `AUTH_PASSWORD_ENABLED` | `"true"` enables password auth. Bootstraps an admin on first start if no users exist. |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` / `GITHUB_REDIRECT_URL` | All three required to enable GitHub OAuth. |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_REDIRECT_URL` | All three required to enable Google OAuth. |
| `OIDC_ISSUER_URL` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` / `OIDC_REDIRECT_URL` | All four required to enable generic OIDC (Okta, Auth0, Azure AD, Keycloak, …). |
| `OIDC_DISPLAY_NAME` | Optional label shown on the SSO button. Defaults to `"SSO"`. |
| `ALLOWED_EMAIL_DOMAINS` | Comma-separated list (e.g. `acme.com,acme.co.uk`). Whitespace and case are normalized. Invalid entries fail loudly at startup. |

### Required
| Var | Default | Purpose |
|---|---|---|
| `CSRF_SECRET_KEY` | (required, ≥ 32 chars) | Signing secret for CSRF tokens. |
| `DATABASE_URL` | (required) | Postgres DSN. |
| `FRONTEND_URL` | (required) | Public origin of the frontend, used in emails and CORS. |
| `ALLOWED_ORIGINS` | (required) | Comma-separated CORS allow-list. |

### Networking / HTTP
| Var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port. |
| `HTTP_READ_TIMEOUT` | `30s` | Server read timeout. |
| `HTTP_WRITE_TIMEOUT` | `30s` | Server write timeout. |

### Email (optional — both must be set to enable)
| Var | Default | Purpose |
|---|---|---|
| `RESEND_API_KEY` | (off) | Resend API key. |
| `EMAIL_FROM_ADDRESS` | (off) | Sender address. |
| `EMAIL_FROM_NAME` | `Confab` | Sender display name. |
| `EMAIL_RATE_LIMIT_PER_HOUR` | `100` | Per-user hourly cap. |

### Storage (S3 / MinIO — all required)
| Var | Default | Purpose |
|---|---|---|
| `S3_ENDPOINT` | (required) | S3-compatible endpoint. |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | (required) | Credentials. |
| `BUCKET_NAME` | (required) | Bucket name. |
| `S3_USE_SSL` | `true` | Set to literal `"false"` to disable TLS (MinIO local dev). Any other value keeps SSL on. |

### Feature flags
| Var | Purpose |
|---|---|
| `SHARE_ALL_SESSIONS_TO_AUTHENTICATED` | `"true"` makes every session visible to all signed-in users. On-prem use. |
| `ENABLE_SHARE_CREATION` | Logs that share links can be created. |
| `ENABLE_SAAS_FOOTER` / `ENABLE_SAAS_TERMLY` | SaaS-only UI/consent toggles. `ENABLE_SAAS_FOOTER=true` also disables the GitHub-release update check (SaaS users can't self-upgrade). |
| `DISABLE_UPDATE_CHECK` | `"true"` suppresses the in-product "Update available" badge by skipping the periodic GitHub release fetch. Useful for air-gapped deployments. |
| `ENABLE_PPROF` | `"true"` exposes `pprof` on `127.0.0.1:6060` (use `fly proxy 6060:6060`). |

### Observability
| Var | Purpose |
|---|---|
| `OTEL_SERVICE_NAME` / `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_EXPORTER_OTLP_HEADERS` | OpenTelemetry config (Honeycomb). Tracing is no-op if unset. |
| `LOG_LEVEL` | `debug` / `info` / `warn` / `error`. Default `info`. |

## Worker env vars

| Var | Default | Purpose |
|---|---|---|
| `WORKER_MAX_SESSIONS` | (required) | Max sessions to scan per cycle for regular cards + smart recap. |
| `WORKER_MAX_SEARCH_INDEX_SESSIONS` | `200` | Max sessions to scan per cycle for search index. |
| `WORKER_POLL_INTERVAL` | `30m` | Cycle interval. Garbage/zero/negative values keep the default. |
| `WORKER_DRY_RUN` | (off) | `"true"` or `"1"` logs intended work without doing it. Case-sensitive. |
| `DATABASE_URL`, S3 vars | (required) | Same as server. |

### Smart recap (LLM-backed)
| Var | Default | Purpose |
|---|---|---|
| `SMART_RECAP_ENABLED` | (off) | `"true"` enables. Silently disabled if `ANTHROPIC_API_KEY` or `SMART_RECAP_MODEL` is missing. |
| `ANTHROPIC_API_KEY` / `SMART_RECAP_MODEL` | (off) | Both required to actually enable. |
| `SMART_RECAP_QUOTA_LIMIT` | unlimited | Per-user-per-month cap. `0` = unlimited. Negative or non-integer fails loudly. |
| `SMART_RECAP_MAX_OUTPUT_TOKENS` | (model default) | Output token cap. |
| `SMART_RECAP_MAX_TRANSCRIPT_TOKENS` | (model default) | Input transcript token cap. |

### Staleness thresholds — `WORKER_REGULAR_*` for regular cards, `WORKER_RECAP_*` for smart recap

Each prefix supports the same five suffixes. Unset values use the per-bucket defaults from `analytics.DefaultRegularCardsThresholds()` / `analytics.DefaultSmartRecapThresholds()`. Out-of-range or unparseable values are silently ignored (defaults stick).

| Suffix | Type | Purpose |
|---|---|---|
| `_THRESHOLD_PCT` | float in [0, 1] | Percentage growth to consider stale. |
| `_BASE_MIN_LINES` | int ≥ 0 | Absolute minimum line gap. |
| `_BASE_MIN_TIME` | duration ≥ 0 | Absolute minimum elapsed time. |
| `_MIN_INITIAL_LINES` | int ≥ 0 | Lines required before any first compute. |
| `_MIN_SESSION_AGE` | duration ≥ 0 | After this age, compute even with few lines. |

## Where to extend

- Adding a new API endpoint: handler in [`internal/api`](../../internal/api); register in `SetupRoutes`; document in [`backend/API.md`](../../API.md).
- Adding analytics cards: follow `/add-session-card` skill — touches `internal/analytics`, migrations, and the frontend.
- Adding a new worker bucket: extend `precomputerAPI` and `Worker.runOnce` in `worker.go` (and the fake in tests). Each bucket has its own `Find*` + `process*` adapter onto `processSessions`.

## Tests

Unit-only (`go test -short` skips integration tests elsewhere; `cmd/server` has no integration tests of its own):

```bash
go test ./cmd/server/...
```

`logFatal` (in `main.go`) is a test seam over `logger.Fatal` so the validation branches are reachable without `os.Exit(1)`. Tests in this package serialize on package-level env and on `logFatal` — do not parallelize.
