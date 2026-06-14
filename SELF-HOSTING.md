# Self-Hosting Guide

Deploy Confabulous — self-hosted analytics for your Claude Code and Codex sessions — on your own infrastructure.

This guide walks you through setup step by step. For the full environment variable reference, see [CONFIGURATION.md](CONFIGURATION.md). For security details, see [backend/SECURITY.md](backend/SECURITY.md).

> **Worked example:** [`confab-demo-site`](https://github.com/ConfabulousDev/confab-demo-site) — live compose, Caddyfile, and OpenTofu behind [demo.confabulous.dev](https://demo.confabulous.dev) on a $7/mo Linode. Tracks this guide.

---

## Prerequisites

- **Docker** and **Docker Compose** v2+
- A server with at least **1 GB RAM** and **1 CPU** (VPS, home lab, cloud VM)
- A **domain name** (optional but recommended for HTTPS)

---

## Quickstart

Get an instance running in under a minute. The repo ships a ready-to-run
`docker-compose.yml` with safe localhost defaults — no configuration needed to
kick the tires.

**1. Clone the repo:**

```bash
git clone https://github.com/ConfabulousDev/confab-web.git
cd confab-web
```

**2. Start the stack:**

```bash
docker compose up -d
```

This pulls the prebuilt image and starts the full stack — app, background worker,
PostgreSQL, and MinIO — wiring up the database and storage bucket automatically.

**3. Open the dashboard:**

Visit [http://localhost:8080](http://localhost:8080) and log in with `admin@local.dev` / `localdevpassword`.

**4. Connect the CLI:**

```bash
curl -fsSL https://raw.githubusercontent.com/ConfabulousDev/confab/main/install.sh | bash
confab setup --backend-url http://localhost:8080
```

Start a Claude Code, Codex, or OpenCode session — it appears in the dashboard automatically.

> The Quickstart defaults are for evaluation only: insecure-cookie mode is on and
> the app is published on `127.0.0.1` (localhost) so it isn't exposed to the
> network. Work through **Production Setup** before exposing it to the internet.

---

## Production Setup

The root `docker-compose.yml` reads every operator-facing value from a `.env`
file next to it. You configure a real deployment by editing `.env` — you don't
edit the compose file.

```bash
cp .env.example .env
```

`.env.example` is organized by section (secrets, URLs, auth, team, smart recaps,
email, …) with every variable documented. Uncomment and set what you need, then
restart with `docker compose up -d`.

### Generate secrets

```bash
openssl rand -base64 32   # CSRF_SECRET_KEY (must be ≥ 32 chars)
openssl rand -base64 24   # each of POSTGRES_PASSWORD / MINIO_ROOT_USER / MINIO_ROOT_PASSWORD
```

Set them in `.env`:

```bash
CSRF_SECRET_KEY=<32+ char random>
POSTGRES_PASSWORD=<random>
MINIO_ROOT_USER=<random>
MINIO_ROOT_PASSWORD=<random>
```

These thread through every service automatically — the bundled Postgres and MinIO
pick them up, and the app's `DATABASE_URL` and S3 credentials are derived from
them. The Quickstart defaults (`confab` / `minioadmin`) are only reachable on the
Docker network, but default credentials are bad hygiene — replace them.

### Set public URLs and turn off dev mode

```bash
FRONTEND_URL=https://confab.example.com
BACKEND_URL=https://confab.example.com
ALLOWED_ORIGINS=https://confab.example.com
INSECURE_DEV_MODE=false
```

All three URLs are typically the same value. They may differ if you run the
frontend and backend on separate domains.

### Admin bootstrap

```bash
ADMIN_BOOTSTRAP_EMAIL=admin@example.com
ADMIN_BOOTSTRAP_PASSWORD=a-strong-password
SUPER_ADMIN_EMAILS=admin@example.com
```

The bootstrap credentials create an admin user on first startup when no users
exist.

> **Important:** After logging in for the first time and confirming your account
> works, remove `ADMIN_BOOTSTRAP_EMAIL` and `ADMIN_BOOTSTRAP_PASSWORD` from `.env`
> and restart. These are only needed for initial setup.

### External PostgreSQL (optional)

To use a managed database (AWS RDS, DigitalOcean, Supabase, etc.) instead of the bundled Postgres:

1. Set `DATABASE_URL` in `.env`:

   ```bash
   DATABASE_URL=postgres://user:password@db-host:5432/confab?sslmode=require
   ```

   (For a separate migration user, also set `MIGRATE_DATABASE_URL`.)

2. Remove the `postgres` service and `postgres_data` volume from `docker-compose.yml`.

### External S3 storage (optional)

To use AWS S3, DigitalOcean Spaces, Wasabi, or another S3-compatible provider instead of MinIO:

1. Set the storage variables in `.env`:

   ```bash
   S3_ENDPOINT=s3.amazonaws.com       # or your provider's endpoint, no http(s):// prefix
   S3_USE_SSL=true
   AWS_ACCESS_KEY_ID=your-access-key
   AWS_SECRET_ACCESS_KEY=your-secret-key
   BUCKET_NAME=your-bucket-name
   ```

2. Remove the `minio`, `minio-setup` services and `minio_data` volume from `docker-compose.yml`.

---

## HTTPS with Caddy

The compose file includes a [Caddy](https://caddyserver.com/) reverse proxy
behind a `caddy` profile. Caddy automatically provisions TLS certificates via
Let's Encrypt — no extra files to add, no port mappings to remove.

**1. Set your domain** in `.env`:

```bash
CONFAB_DOMAIN=confab.example.com
FRONTEND_URL=https://confab.example.com
BACKEND_URL=https://confab.example.com
ALLOWED_ORIGINS=https://confab.example.com
INSECURE_DEV_MODE=false
```

**2. Point your DNS** A record at your server's IP.

**3. Start with the Caddy profile:**

```bash
docker compose --profile caddy up -d
```

Caddy obtains a certificate for `CONFAB_DOMAIN` and reverse-proxies it to the app.
The bundled [`Caddyfile`](Caddyfile) handles TLS, gzip/zstd compression, and
rotating access logs; edit it only if you need custom proxy behavior.

---

## Authentication

At least one authentication method must be enabled. You can enable multiple methods simultaneously. All of these go in `.env`.

### Password Auth

The simplest option — recommended for single-user or small team deployments. On by default:

```bash
AUTH_PASSWORD_ENABLED=true
```

### GitHub OAuth

Create an OAuth app at [github.com/settings/developers](https://github.com/settings/developers):
- **Homepage URL:** `https://confab.example.com`
- **Authorization callback URL:** `https://confab.example.com/auth/github/callback`

```bash
GITHUB_CLIENT_ID=your-client-id
GITHUB_CLIENT_SECRET=your-client-secret
GITHUB_REDIRECT_URL=https://confab.example.com/auth/github/callback
```

### Google OAuth

Create OAuth credentials at [console.cloud.google.com/apis/credentials](https://console.cloud.google.com/apis/credentials):
- **Authorized redirect URI:** `https://confab.example.com/auth/google/callback`

```bash
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=https://confab.example.com/auth/google/callback
```

### Generic OIDC

Works with Keycloak, Okta, Auth0, Azure AD, and any OpenID Connect provider. The provider must support OIDC Discovery (`/.well-known/openid-configuration`). All four variables must be set:

```bash
OIDC_ISSUER_URL=https://your-idp.example.com
OIDC_CLIENT_ID=your-client-id
OIDC_CLIENT_SECRET=your-client-secret
OIDC_REDIRECT_URL=https://confab.example.com/auth/oidc/callback
OIDC_DISPLAY_NAME=SSO  # Controls button text ("Continue with ...")
```

---

## Single-Tenant / Single-Org Lockdown

For an internal-only instance with no public signups, two variables lock the deployment down. Set both in `.env` for a fully closed instance.

**Restrict who can log in** (applies to password, OAuth, and OIDC):

```bash
ALLOWED_EMAIL_DOMAINS=company.com,partner.com
```

**Block new registrations** (existing users keep working; new sign-ups are rejected):

```bash
MAX_USERS=0
```

---

## Team Settings

Configure how your team interacts with sessions and sharing (all in `.env`).

| Variable | What it does |
|----------|-------------|
| `SHARE_ALL_SESSIONS_TO_AUTHENTICATED` | Set to `true` to make every session visible to all authenticated users. Useful for small teams that want full transparency. |
| `ENABLE_SHARE_CREATION` | Set to `true` to allow users to create external share links. |
| `ENABLE_ORG_ANALYTICS` | Set to `true` to expose org-wide per-user analytics (`/admin/...`) to every authenticated user — same visibility model as `SHARE_ALL_SESSIONS_TO_AUTHENTICATED`. See [Organization Analytics in backend/API.md](backend/API.md#organization-analytics) for the privacy implications. |
| `MAX_USERS` | Maximum registered users (default: `50`). Set to `0` to block new registrations. |
| `SUPER_ADMIN_EMAILS` | Comma-separated emails with access to the admin panel at `/admin/users`. |

---

## Customization

| Variable | What it does |
|----------|-------------|
| `SUPPORT_EMAIL` | Support email address shown in the UI. |

### Email (for Share Invitations)

To enable email notifications for share invitations, sign up at [resend.com](https://resend.com) and add to `.env`:

```bash
RESEND_API_KEY=re_xxxxxxxxxxxx
EMAIL_FROM_ADDRESS=noreply@example.com
```

See [CONFIGURATION.md](CONFIGURATION.md) for additional email settings.

---

## Connecting the CLI

Install the [Confab CLI](https://github.com/ConfabulousDev/confab):

```bash
curl -fsSL https://raw.githubusercontent.com/ConfabulousDev/confab/main/install.sh | bash
```

Then configure it to point at your instance:

```bash
confab setup --backend-url https://confab.example.com
```

The install script fetches pre-built binaries from [GitHub Releases](https://github.com/ConfabulousDev/confab/releases). Start a Claude Code or Codex session and it will appear in your dashboard automatically.

---

## Smart Recaps (Optional)

AI-powered session summaries using the Anthropic API. Requires an [Anthropic API key](https://console.anthropic.com/). Add to `.env`:

```bash
SMART_RECAP_ENABLED=true
ANTHROPIC_API_KEY=sk-ant-xxxxxxxxxxxx
SMART_RECAP_MODEL=claude-haiku-4-5-20251001
SMART_RECAP_QUOTA_LIMIT=500  # Monthly per-user generation limit
```

The bundled `worker` service precomputes recaps in the background. See [CONFIGURATION.md](CONFIGURATION.md) for advanced worker tuning options.

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

Migrations are idempotent — safe to run multiple times. The `migrate` service exits after completion. (If you run with HTTPS, keep using `--profile caddy` on the `up` command.)

---

## Troubleshooting

### CORS errors in the browser console

`ALLOWED_ORIGINS` must exactly match the URL in your browser's address bar, including the scheme (`https://`) and port (if non-standard). No trailing slash.

### OAuth callback fails with "redirect URI mismatch"

The redirect URL in your OAuth provider's settings must exactly match the environment variable (`GITHUB_REDIRECT_URL`, `GOOGLE_REDIRECT_URL`, or `OIDC_REDIRECT_URL`), including the scheme and path.

### S3 / MinIO connection errors

- `S3_ENDPOINT` should **not** include `http://` or `https://` — just the host and port (e.g., `minio:9000`).
- Set `S3_USE_SSL` to `false` for local MinIO, `true` for external providers.
- Ensure the bucket exists. The `minio-setup` service creates it automatically for local MinIO.

### "No authentication methods enabled"

At least one auth method must be configured. Set `AUTH_PASSWORD_ENABLED=true` or configure an OAuth/OIDC provider.

### Cookies not persisting / login loop

Without HTTPS, you must set `INSECURE_DEV_MODE=true`. In production, use HTTPS (see [HTTPS with Caddy](#https-with-caddy)) and ensure `INSECURE_DEV_MODE` is `false`.

### Database connection refused

- Verify the `DATABASE_URL` is correct and the Postgres server is reachable from the Docker network.
- If using the bundled Postgres, ensure the `postgres` service is healthy: `docker compose ps`.

### Port 8080 already in use

Set `PORT` in `.env` to a free port — the published localhost port follows it automatically.

---

## Security Checklist

Before exposing your instance to the internet:

- [ ] `INSECURE_DEV_MODE` is `false`
- [ ] `CSRF_SECRET_KEY` is a unique random string (32+ characters)
- [ ] `POSTGRES_PASSWORD` and `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` are random values, not the Quickstart defaults
- [ ] `ALLOWED_ORIGINS` contains only your domain
- [ ] HTTPS is enforced (via the Caddy profile or another reverse proxy)
- [ ] Bootstrap credentials (`ADMIN_BOOTSTRAP_*`) are removed after setup
- [ ] Database uses SSL (`sslmode=require` in `DATABASE_URL`) if external
- [ ] OAuth secrets are production values (not development/test credentials)

For a comprehensive security review, see [backend/SECURITY.md](backend/SECURITY.md).
