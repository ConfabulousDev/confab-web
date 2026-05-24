---
title: Sample deployments
description: Annotated, real-world deployments of Confabulous you can copy from.
---

The Confabulous-maintained instances run on two different stacks. Both configurations are public and you can copy from them.

## Fly.io — `confabulous.dev` (managed)

The free managed instance at [confabulous.dev](https://confabulous.dev) runs on [Fly.io](https://fly.io/), using:

- **App + worker** as separate Fly processes from a single image.
- **Tigris** ([Fly Object Storage](https://fly.io/docs/reference/tigris/)) for S3-compatible session-blob storage.
- **[Neon](https://neon.tech/)** as the managed Postgres provider — connection string passed in as `DATABASE_URL` (a Fly secret).
- **Resend** for share-invitation email.
- **Honeycomb** for OpenTelemetry traces.
- **Anthropic** for Smart Recaps.

The full configuration lives in the repo at [`fly.toml`](https://github.com/ConfabulousDev/confab-web/blob/main/fly.toml), with the deploy script at [`deploy-to-fly.sh`](https://github.com/ConfabulousDev/confab-web/blob/main/deploy-to-fly.sh).

Notable choices for a Fly.io deployment:

- Two `[[vm]]` blocks — one for the `app` process (auto-stop enabled), one for the always-on `worker` singleton.
- `S3_ENDPOINT = "fly.storage.tigris.dev"` for Tigris, with `S3_USE_SSL = "true"`.
- Secrets (`AWS_*`, `CSRF_SECRET_KEY`, `RESEND_API_KEY`, `ANTHROPIC_API_KEY`, `OTEL_EXPORTER_OTLP_HEADERS`) are set via `fly secrets set`, not in `fly.toml`.
- `auto_stop_machines = 'stop'` plus `min_machines_running = 1` keeps response latency low while letting unused machines stop on idle.

### Deploying

```bash
# One-time:
fly launch  # or `fly deploy` if the app already exists

# Run database migrations against the production DB, then deploy:
export PRODUCTION_DATABASE_URL='postgresql://user:pass@host/db?sslmode=require'
./deploy-to-fly.sh
```

## Linode — `demo.confabulous.dev` (demo)

The public demo instance at [demo.confabulous.dev](https://demo.confabulous.dev) runs on a [Linode](https://www.linode.com/) VPS using a Docker Compose stack much like the one in the [Deployment walkthrough](/self-hosting/deploy/), with these additions:

- **Caddy** in front of the `app` service for HTTPS via Let's Encrypt.
- **Demo mode** enabled via `DEMO_IDENTITY_EMAIL=demo@confabulous.dev` — see [Demo mode](/self-hosting/demo-mode/).
- **Auth:** password auth (`AUTH_PASSWORD_ENABLED=true`) — anonymous visitors are auto-impersonated as the read-only demo user, and the operator signs in as a real account with a password. OAuth is intentionally not configured.

:::note
The exact `docker-compose.yml` and `Caddyfile` for the demo deployment are not yet published. If you'd like to mirror this configuration, start from the [Deployment walkthrough](/self-hosting/deploy/) and enable [Demo mode](/self-hosting/demo-mode/) — the two together produce the same result.
:::

## Picking a stack

| If you want… | Use |
|--|--|
| Managed PaaS, scale-to-zero, minimal ops | **Fly.io** (or similar — Railway, Render) |
| Full control of a VM, low monthly cost | **Linode VPS** (or DigitalOcean, Hetzner) + Docker Compose + Caddy |
| Enterprise infra, OIDC SSO | Bring your own — k8s, ECS, etc. Confabulous is just a single Docker image plus Postgres + S3. |
