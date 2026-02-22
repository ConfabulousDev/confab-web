# Self-Hosting Guide

Deploy Confabulous — self-hosted analytics for your Claude Code sessions — on your own infrastructure.

This guide walks you through setup step by step. For the full environment variable reference, see [CONFIGURATION.md](CONFIGURATION.md). For security details, see [backend/SECURITY.md](backend/SECURITY.md).

---

## Prerequisites

- **Docker** and **Docker Compose** v2+
- A server with at least **1 GB RAM** and **1 CPU** (VPS, home lab, cloud VM)
- A **domain name** (optional but recommended for HTTPS)

---

## Quickstart

Get a local instance running in under a minute.

**1. Create a project directory:**

```bash
mkdir confabulous && cd confabulous
```

**2. Create `docker-compose.yml`:**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: confab
      POSTGRES_PASSWORD: confab
      POSTGRES_DB: confab
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U confab"]
      interval: 5s
      timeout: 3s
      retries: 5

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 5s
      timeout: 3s
      retries: 5

  minio-setup:
    image: minio/mc:latest
    depends_on:
      minio:
        condition: service_healthy
    entrypoint: >
      /bin/sh -c "
      /usr/bin/mc alias set minio http://minio:9000 minioadmin minioadmin;
      /usr/bin/mc mb minio/confab --ignore-existing;
      exit 0;
      "

  migrate:
    image: ghcr.io/confabulousdev/confab-web:latest
    depends_on:
      postgres:
        condition: service_healthy
    command: ["./migrate_db.sh"]
    environment:
      DATABASE_URL: postgres://confab:confab@postgres:5432/confab?sslmode=disable

  app:
    image: ghcr.io/confabulousdev/confab-web:latest
    depends_on:
      migrate:
        condition: service_completed_successfully
      minio-setup:
        condition: service_completed_successfully
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      PORT: 8080
      DATABASE_URL: postgres://confab:confab@postgres:5432/confab?sslmode=disable
      S3_ENDPOINT: minio:9000
      S3_USE_SSL: "false"
      AWS_ACCESS_KEY_ID: minioadmin
      AWS_SECRET_ACCESS_KEY: minioadmin
      BUCKET_NAME: confab
      FRONTEND_URL: http://localhost:8080
      BACKEND_URL: http://localhost:8080
      ALLOWED_ORIGINS: http://localhost:8080
      CSRF_SECRET_KEY: local-dev-csrf-secret-change-me-32chars
      AUTH_PASSWORD_ENABLED: "true"
      ADMIN_BOOTSTRAP_EMAIL: admin@local.dev
      ADMIN_BOOTSTRAP_PASSWORD: localdevpassword
      SUPER_ADMIN_EMAILS: admin@local.dev
      INSECURE_DEV_MODE: "true"

  worker:
    image: ghcr.io/confabulousdev/confab-web:latest
    command: ["./confab", "worker"]
    depends_on:
      migrate:
        condition: service_completed_successfully
      minio-setup:
        condition: service_completed_successfully
    environment:
      DATABASE_URL: postgres://confab:confab@postgres:5432/confab?sslmode=disable
      S3_ENDPOINT: minio:9000
      S3_USE_SSL: "false"
      AWS_ACCESS_KEY_ID: minioadmin
      AWS_SECRET_ACCESS_KEY: minioadmin
      BUCKET_NAME: confab
      WORKER_POLL_INTERVAL: 1m
      WORKER_MAX_SESSIONS: "10"

volumes:
  postgres_data:
  minio_data:
```

**3. Start the stack:**

```bash
docker compose up -d
```

**4. Open the dashboard:**

Visit [http://localhost:8080](http://localhost:8080) and log in with `admin@local.dev` / `localdevpassword`.

**5. Connect the CLI:**

```bash
curl -fsSL http://localhost:8080/install | bash
confab setup --backend-url http://localhost:8080
```

Start a Claude Code session — it appears in the dashboard automatically.

---

## Production Setup

To run Confabulous on a server with a real domain, customize the environment variables in your `docker-compose.yml`.

### Generate Secrets

Replace the placeholder CSRF key with a random value:

```bash
openssl rand -base64 32
```

Set the result as `CSRF_SECRET_KEY` in the `app` service. Choose a strong admin password and update `ADMIN_BOOTSTRAP_EMAIL` and `ADMIN_BOOTSTRAP_PASSWORD`.

Remove or set to `"false"`:

```yaml
INSECURE_DEV_MODE: "false"
```

> **Important:** After logging in for the first time and confirming your account works, remove `ADMIN_BOOTSTRAP_EMAIL` and `ADMIN_BOOTSTRAP_PASSWORD` from the compose file and restart. These are only needed for initial setup.

### Set Public URLs

Update the URL variables in the `app` service to your domain:

```yaml
FRONTEND_URL: https://confab.example.com
BACKEND_URL: https://confab.example.com
ALLOWED_ORIGINS: https://confab.example.com
```

All three are typically the same value. They may differ if you run the frontend and backend on separate domains.

### External PostgreSQL (Optional)

To use a managed database (AWS RDS, DigitalOcean, Supabase, etc.) instead of the bundled Postgres:

1. Update `DATABASE_URL` in **both** the `app` and `worker` services:

```yaml
DATABASE_URL: postgres://user:password@db-host:5432/confab?sslmode=require
```

2. Update `DATABASE_URL` in the `migrate` service to match (or use `MIGRATE_DATABASE_URL` for a separate admin user).

3. Remove the `postgres` service and `postgres_data` volume from the compose file.

### External S3 Storage (Optional)

To use AWS S3, DigitalOcean Spaces, Wasabi, or another S3-compatible provider instead of MinIO:

1. Update the storage variables in **both** the `app` and `worker` services:

```yaml
S3_ENDPOINT: s3.amazonaws.com       # or your provider's endpoint
S3_USE_SSL: "true"
AWS_ACCESS_KEY_ID: your-access-key
AWS_SECRET_ACCESS_KEY: your-secret-key
BUCKET_NAME: your-bucket-name
```

2. Remove the `minio`, `minio-setup` services and `minio_data` volume from the compose file.

> **Note:** The `S3_ENDPOINT` should not include the `http://` or `https://` protocol prefix.

---

## HTTPS with Caddy

[Caddy](https://caddyserver.com/) automatically provisions TLS certificates via Let's Encrypt. Add it to your compose stack for zero-config HTTPS.

**1. Add a Caddy service** to your `docker-compose.yml`:

```yaml
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - app
```

Add the volumes to the `volumes:` section at the bottom:

```yaml
volumes:
  postgres_data:
  minio_data:
  caddy_data:
  caddy_config:
```

**2. Remove the port mapping** from the `app` service (Caddy handles external traffic):

```yaml
  app:
    # Remove this line:
    # ports:
    #   - "127.0.0.1:8080:8080"
```

**3. Create a `Caddyfile`** in the same directory:

```
confab.example.com {
    reverse_proxy app:8080
}
```

Replace `confab.example.com` with your domain.

**4. Update environment variables** in the `app` service:

```yaml
FRONTEND_URL: https://confab.example.com
BACKEND_URL: https://confab.example.com
ALLOWED_ORIGINS: https://confab.example.com
INSECURE_DEV_MODE: "false"
```

**5. Point your DNS** A record to your server's IP address, then restart:

```bash
docker compose up -d
```

Caddy will automatically obtain a TLS certificate for your domain.

---

## Authentication

At least one authentication method must be enabled. You can enable multiple methods simultaneously.

### Password Auth

The simplest option — recommended for single-user or small team deployments.

```yaml
AUTH_PASSWORD_ENABLED: "true"
ADMIN_BOOTSTRAP_EMAIL: admin@example.com
ADMIN_BOOTSTRAP_PASSWORD: a-strong-password
```

The bootstrap credentials create an admin user on first startup when no users exist. Remove them from the compose file after initial setup.

### GitHub OAuth

Create an OAuth app at [github.com/settings/developers](https://github.com/settings/developers):
- **Homepage URL:** `https://confab.example.com`
- **Authorization callback URL:** `https://confab.example.com/auth/github/callback`

Add to the `app` service:

```yaml
GITHUB_CLIENT_ID: your-client-id
GITHUB_CLIENT_SECRET: your-client-secret
GITHUB_REDIRECT_URL: https://confab.example.com/auth/github/callback
```

### Google OAuth

Create OAuth credentials at [console.cloud.google.com/apis/credentials](https://console.cloud.google.com/apis/credentials):
- **Authorized redirect URI:** `https://confab.example.com/auth/google/callback`

Add to the `app` service:

```yaml
GOOGLE_CLIENT_ID: your-client-id
GOOGLE_CLIENT_SECRET: your-client-secret
GOOGLE_REDIRECT_URL: https://confab.example.com/auth/google/callback
```

### Generic OIDC

Works with Keycloak, Okta, Auth0, Azure AD, and any OpenID Connect provider. The provider must support OIDC Discovery (`/.well-known/openid-configuration`).

Add to the `app` service:

```yaml
OIDC_ISSUER_URL: https://your-idp.example.com
OIDC_CLIENT_ID: your-client-id
OIDC_CLIENT_SECRET: your-client-secret
OIDC_REDIRECT_URL: https://confab.example.com/auth/oidc/callback
OIDC_DISPLAY_NAME: SSO  # Controls button text ("Continue with ...")
```

All four variables (`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`) must be set to enable OIDC.

### Email Domain Restriction

Restrict login to specific email domains (applies to all auth methods):

```yaml
ALLOWED_EMAIL_DOMAINS: company.com,partner.com
```

---

## Team Settings

Configure how your team interacts with sessions and sharing.

| Variable | What it does |
|----------|-------------|
| `SHARE_ALL_SESSIONS_TO_AUTHENTICATED` | Set to `"true"` to make every session visible to all authenticated users. Useful for small teams that want full transparency. |
| `DISABLE_SHARE_CREATION` | Set to `"true"` to prevent users from creating external share links. |
| `MAX_USERS` | Maximum registered users (default: `50`). Set to `"0"` to block new registrations. |
| `SUPER_ADMIN_EMAILS` | Comma-separated emails with access to the admin panel at `/admin/users`. |

---

## Customization

| Variable | What it does |
|----------|-------------|
| `DISABLE_FOOTER` | Set to `"true"` to hide the default footer (GitHub, Discord, Help links). |
| `DISABLE_TERMLY` | Set to `"true"` to remove the Termly cookie-consent banner. |
| `SUPPORT_EMAIL` | Support email address shown in the UI. |

### Email (for Share Invitations)

To enable email notifications for share invitations, sign up at [resend.com](https://resend.com) and add:

```yaml
RESEND_API_KEY: re_xxxxxxxxxxxx
EMAIL_FROM_ADDRESS: noreply@example.com
```

See [CONFIGURATION.md](CONFIGURATION.md) for additional email settings.

---

## Connecting the CLI

Install the [Confab CLI](https://github.com/ConfabulousDev/confab) from your running server:

```bash
curl -fsSL https://confab.example.com/install | bash
```

Then configure it to point at your instance:

```bash
confab setup --backend-url https://confab.example.com
```

The install script fetches pre-built binaries from [GitHub Releases](https://github.com/ConfabulousDev/confab/releases). Start a Claude Code session and it will appear in your dashboard automatically.

---

## Smart Recaps (Optional)

AI-powered session summaries using the Anthropic API. Requires an [Anthropic API key](https://console.anthropic.com/).

Add to **both** the `app` and `worker` services:

```yaml
SMART_RECAP_ENABLED: "true"
ANTHROPIC_API_KEY: sk-ant-xxxxxxxxxxxx
SMART_RECAP_MODEL: claude-haiku-4-5-20251001
SMART_RECAP_QUOTA_LIMIT: "500"  # Monthly generation limit
```

The `worker` service (already in the quickstart compose file) precomputes recaps in the background. See [CONFIGURATION.md](CONFIGURATION.md) for advanced worker tuning options.

---

## Upgrading

When a new version is released:

```bash
# 1. Pull the latest images
docker compose pull

# 2. Run database migrations
docker compose run --rm migrate

# 3. Restart services with the new images
docker compose up -d
```

Migrations are idempotent — safe to run multiple times. The `migrate` service exits after completion.

---

## Troubleshooting

### CORS errors in the browser console

`ALLOWED_ORIGINS` must exactly match the URL in your browser's address bar, including the scheme (`https://`) and port (if non-standard). No trailing slash.

### OAuth callback fails with "redirect URI mismatch"

The redirect URL in your OAuth provider's settings must exactly match the environment variable (`GITHUB_REDIRECT_URL`, `GOOGLE_REDIRECT_URL`, or `OIDC_REDIRECT_URL`), including the scheme and path.

### S3 / MinIO connection errors

- `S3_ENDPOINT` should **not** include `http://` or `https://` — just the host and port (e.g., `minio:9000`).
- Set `S3_USE_SSL` to `"false"` for local MinIO, `"true"` for external providers.
- Ensure the bucket exists. The `minio-setup` service creates it automatically for local MinIO.

### "No authentication methods enabled"

At least one auth method must be configured. Set `AUTH_PASSWORD_ENABLED: "true"` or configure an OAuth/OIDC provider.

### Cookies not persisting / login loop

Without HTTPS, you must set `INSECURE_DEV_MODE: "true"`. In production, use HTTPS (see [HTTPS with Caddy](#https-with-caddy)) and ensure `INSECURE_DEV_MODE` is unset or `"false"`.

### Database connection refused

- Verify the `DATABASE_URL` is correct and the Postgres server is reachable from the Docker network.
- If using the bundled Postgres, ensure the `postgres` service is healthy: `docker compose ps`.

### Port 8080 already in use

Change `PORT` in the `app` service and update the port mapping to match:

```yaml
ports:
  - "127.0.0.1:3000:3000"
environment:
  PORT: "3000"
```

---

## Security Checklist

Before exposing your instance to the internet:

- [ ] `INSECURE_DEV_MODE` is unset or `"false"`
- [ ] `CSRF_SECRET_KEY` is a unique random string (32+ characters)
- [ ] `ALLOWED_ORIGINS` contains only your domain
- [ ] HTTPS is enforced (via Caddy or another reverse proxy)
- [ ] Bootstrap credentials (`ADMIN_BOOTSTRAP_*`) are removed after setup
- [ ] Database uses SSL (`sslmode=require` in `DATABASE_URL`) if external
- [ ] OAuth secrets are production values (not development/test credentials)

For a comprehensive security review, see [backend/SECURITY.md](backend/SECURITY.md).
