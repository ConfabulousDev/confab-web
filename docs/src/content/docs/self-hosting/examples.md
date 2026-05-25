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

[demo.confabulous.dev](https://demo.confabulous.dev) runs on a ~$7/month [Linode](https://www.linode.com/) Nanode (1 GB + backups). The full infra-as-code lives in [`ConfabulousDev/confab-demo-site`](https://github.com/ConfabulousDev/confab-demo-site) — the [Deployment walkthrough](/self-hosting/deploy/) stack plus [Demo mode](/self-hosting/demo-mode/), Caddy for HTTPS, and `MAX_USERS=0`.

What's in the repo:

- [`stack/`](https://github.com/ConfabulousDev/confab-demo-site/tree/main/stack) — `docker-compose.yml` and `Caddyfile`.
- [`tofu/`](https://github.com/ConfabulousDev/confab-demo-site/tree/main/tofu) — [OpenTofu](https://opentofu.org/) for the Linode VM and firewall. [`cloud-init.yaml.tftpl`](https://github.com/ConfabulousDev/confab-demo-site/blob/main/tofu/cloud-init.yaml.tftpl) renders `stack/` plus secrets into `/opt/confab/` on first boot and starts the stack via systemd. `ignore_changes = [metadata]` keeps later `apply`s from rebuilding the VM.
- [`scripts/deploy.sh`](https://github.com/ConfabulousDev/confab-demo-site/blob/main/scripts/deploy.sh) — image and compose updates. Infra goes through `tofu apply`; secrets are edited in `/opt/confab/.env` on the VM.

Cloudflare DNS is added by hand so no DNS token lives in the repo.

## Picking a stack

| If you want… | Use |
|--|--|
| Managed PaaS, scale-to-zero, minimal ops | **Fly.io** (or similar — Railway, Render) |
| Full control of a VM, low monthly cost | **Linode VPS** (or DigitalOcean, Hetzner) + Docker Compose + Caddy |
| Enterprise infra, OIDC SSO | Bring your own — k8s, ECS, etc. Confabulous is just a single Docker image plus Postgres + S3. |
