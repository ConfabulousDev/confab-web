# Configuration Reference

All configuration is via environment variables, set in `docker-compose.yml` or your platform's equivalent. The backend runs as either `./server` (web) or `./server worker` (background analytics). Which mode each variable applies to is noted per section.

See [`backend/.env.example`](backend/.env.example) for a copy-paste-ready template.

---

## Core Settings

*Applies to: web server*

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PORT` | `8080` | No | HTTP server port |
| `BACKEND_URL` | `http://localhost:8080` | No | Public URL of the backend; used for CLI device-code authorization flow |
| `FRONTEND_URL` | `http://localhost:5173` | Yes | Public URL of the frontend; used for OAuth redirects, email links, and CORS |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | Yes | Comma-separated list of allowed CORS origins |
| `STATIC_FILES_DIR` | `/app/static` | No | Directory for serving the built frontend |
| `CSRF_SECRET_KEY` | *(none)* | Yes | CSRF protection secret; must be at least 32 characters |
| `INSECURE_DEV_MODE` | `false` | No | Set to `true` to disable secure-cookie requirements (local dev without HTTPS) |

## Database

*Applies to: web server and worker*

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `DATABASE_URL` | *(none)* | Yes | PostgreSQL connection string (e.g. `postgres://user:pass@host:5432/confab?sslmode=disable`) |
| `MIGRATE_DATABASE_URL` | Falls back to `DATABASE_URL` | No | Separate connection string for running migrations with an elevated database user |

## Storage

*Applies to: web server and worker*

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `S3_ENDPOINT` | *(none)* | Yes | S3/MinIO endpoint (e.g. `localhost:9000`) |
| `AWS_ACCESS_KEY_ID` | *(none)* | Yes | S3/MinIO access key |
| `AWS_SECRET_ACCESS_KEY` | *(none)* | Yes | S3/MinIO secret key |
| `BUCKET_NAME` | *(none)* | Yes | S3/MinIO bucket name |
| `S3_USE_SSL` | `true` | No | Use SSL for S3 connections; set to `false` for local MinIO |

## Authentication

*Applies to: web server*

At least one method must be enabled. All four can be used simultaneously.

### Password Auth

Recommended for self-hosted deployments.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `AUTH_PASSWORD_ENABLED` | `false` | No | Set to `true` to enable username/password login |
| `ADMIN_BOOTSTRAP_EMAIL` | *(none)* | If password auth enabled | Email for the initial admin user (created on first startup if no users exist) |
| `ADMIN_BOOTSTRAP_PASSWORD` | *(none)* | If password auth enabled | Password for the initial admin user; remove after setup |

### GitHub OAuth

Create an OAuth app at [github.com/settings/developers](https://github.com/settings/developers).

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `GITHUB_CLIENT_ID` | *(none)* | If GitHub OAuth enabled | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | *(none)* | If GitHub OAuth enabled | GitHub OAuth app client secret |
| `GITHUB_REDIRECT_URL` | *(none)* | If GitHub OAuth enabled | OAuth callback URL (e.g. `https://your-domain/auth/github/callback`) |

### Google OAuth

Create OAuth credentials at [console.cloud.google.com/apis/credentials](https://console.cloud.google.com/apis/credentials).

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `GOOGLE_CLIENT_ID` | *(none)* | If Google OAuth enabled | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | *(none)* | If Google OAuth enabled | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | *(none)* | If Google OAuth enabled | OAuth callback URL (e.g. `https://your-domain/auth/google/callback`) |

### Generic OIDC

Works with Okta, Auth0, Azure AD, Keycloak, etc. All four variables must be set.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `OIDC_ISSUER_URL` | *(none)* | If OIDC enabled | OIDC issuer URL (e.g. `https://dev-12345.okta.com`) |
| `OIDC_CLIENT_ID` | *(none)* | If OIDC enabled | OIDC client ID |
| `OIDC_CLIENT_SECRET` | *(none)* | If OIDC enabled | OIDC client secret |
| `OIDC_REDIRECT_URL` | *(none)* | If OIDC enabled | OAuth callback URL (e.g. `https://your-domain/auth/oidc/callback`) |
| `OIDC_DISPLAY_NAME` | `SSO` | No | Controls the login button text ("Continue with ...") |

### Shared Auth Settings

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `ALLOWED_EMAIL_DOMAINS` | *(all domains)* | No | Comma-separated list of allowed email domains; applies to all auth methods |

## Email

*Applies to: web server*

Email is enabled when both `RESEND_API_KEY` and `EMAIL_FROM_ADDRESS` are set.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `RESEND_API_KEY` | *(none)* | If email enabled | Resend API key ([resend.com](https://resend.com)) |
| `EMAIL_FROM_ADDRESS` | *(none)* | If email enabled | Sender email address |
| `EMAIL_FROM_NAME` | `Confab` | No | Sender display name |
| `EMAIL_RATE_LIMIT_PER_HOUR` | `100` | No | Per-user email rate limit |
| `SUPPORT_EMAIL` | *(none)* | No | Support email shown in UI |

## Smart Recaps

*Applies to: web server and worker*

AI-powered session summaries. Requires an [Anthropic API key](https://console.anthropic.com/).

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SMART_RECAP_ENABLED` | `false` | No | Set to `true` to enable smart recaps |
| `ANTHROPIC_API_KEY` | *(none)* | If smart recaps enabled | Anthropic API key |
| `SMART_RECAP_MODEL` | *(none)* | If smart recaps enabled | Model to use (e.g. `claude-haiku-4-5-20251001`) |
| `SMART_RECAP_QUOTA_LIMIT` | `0` (unlimited) | No | Per-user monthly generation cap. Positive integer enforces a limit; `0` or omitted means unlimited. |
| `SMART_RECAP_MAX_OUTPUT_TOKENS` | `1000` | No | Maximum LLM output tokens per recap |
| `SMART_RECAP_MAX_TRANSCRIPT_TOKENS` | `50000` | No | Maximum input tokens per transcript (~chars/4) |

## Admin & User Management

*Applies to: web server*

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SUPER_ADMIN_EMAILS` | *(none)* | No | Comma-separated email addresses with admin panel access |
| `MAX_USERS` | `50` | No | Maximum number of registered users; set to `0` to block new registrations |

## Instance Customization

*Applies to: web server*

Sharing behavior and UI customization.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SHARE_ALL_SESSIONS_TO_AUTHENTICATED` | `false` | No | Make every session visible to every authenticated user; useful for small teams that want full transparency |
| `ENABLE_SHARE_CREATION` | `false` | No | Enable share link creation |
| `ENABLE_SAAS_FOOTER` | `false` | No | Show the SaaS footer (GitHub, Discord, Help links, copyright); off by default for self-hosted |
| `ENABLE_SAAS_TERMLY` | `false` | No | Enable the Termly cookie-consent banner (SaaS only); off by default for self-hosted |

## Worker

*Applies to: worker only*

Precomputes analytics and smart recaps in the background.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `WORKER_POLL_INTERVAL` | `30m` | No | How often to check for stale sessions |
| `WORKER_MAX_SESSIONS` | `20` | No | Maximum sessions to process per cycle |
| `WORKER_DRY_RUN` | `false` | No | Log what would be done without actually processing |

### Staleness Thresholds (Advanced)

Controls when sessions need recomputation. `WORKER_REGULAR_*` = analytics cards, `WORKER_RECAP_*` = smart recaps.

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_REGULAR_THRESHOLD_PCT` | `0.20` | Percentage change (0–1) to trigger analytics recompute |
| `WORKER_REGULAR_BASE_MIN_LINES` | `5` | Minimum new lines before recompute |
| `WORKER_REGULAR_BASE_MIN_TIME` | `3m` | Minimum age of new data |
| `WORKER_REGULAR_MIN_INITIAL_LINES` | `10` | Minimum lines for first computation |
| `WORKER_REGULAR_MIN_SESSION_AGE` | `10m` | Minimum session age |
| `WORKER_RECAP_THRESHOLD_PCT` | `0.20` | Percentage change (0–1) to trigger recap recompute |
| `WORKER_RECAP_BASE_MIN_LINES` | `150` | Minimum new lines before recompute |
| `WORKER_RECAP_BASE_MIN_TIME` | `30m` | Minimum age of new data |
| `WORKER_RECAP_MIN_INITIAL_LINES` | `25` | Minimum lines for first computation |
| `WORKER_RECAP_MIN_SESSION_AGE` | `10m` | Minimum session age |

## Observability

*Applies to: web server and worker*

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `LOG_LEVEL` | `info` | No | Log level: `debug`, `info`, `warn`, `error` |
| `OTEL_SERVICE_NAME` | *(none)* | No | OpenTelemetry service name |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(none)* | No | OTLP exporter endpoint (e.g. `https://api.honeycomb.io`) |
| `OTEL_EXPORTER_OTLP_HEADERS` | *(none)* | No | OTLP exporter headers (e.g. `x-honeycomb-team=your-api-key`) |
| `ENABLE_PPROF` | `false` | No | Enable pprof profiling server on `localhost:6060` |

## HTTP Tuning

*Applies to: web server*

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `HTTP_READ_TIMEOUT` | `30s` | No | HTTP read timeout |
| `HTTP_WRITE_TIMEOUT` | `30s` | No | HTTP write timeout |
