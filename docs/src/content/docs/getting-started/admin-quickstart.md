---
title: Quickstart for admins
description: Stand up a Confabulous Code stack — locally for prototyping, then on real infrastructure for your team.
---

This page is for the person who runs the Confabulous Code instance — provisioning the stack, configuring auth, and operating it for end users. If your team already has a Confabulous Code instance and you just want to use it, see the [Quickstart for end users](/getting-started/end-user-quickstart/).

## 1. Try locally

The fastest way to see Confabulous Code working end-to-end is to clone the repo and run the full stack on your laptop with Docker Compose. This is purely for evaluation — it is **not** a production deployment.

**Prerequisites:** Docker and Docker Compose, plus `git`.

```bash
git clone https://github.com/ConfabulousDev/confab-web.git
cd confab-web
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080) and log in with:

- **Email:** `admin@local.dev`
- **Password:** `localdevpassword`

:::caution
These default credentials and the localhost binding are for evaluation only. Never expose this configuration to a network. When you're ready for a real deployment, see [Deployment walkthrough](/self-hosting/deploy/).
:::

Try the [Quickstart for end users](/getting-started/end-user-quickstart/) flow against your local instance (using `--backend-url http://localhost:8080`) to confirm the CLI streams sessions in.

## 2. Deploy for real

Localhost is for prototyping; a real Confabulous Code instance lives on real infrastructure with HTTPS, real authentication, and a real domain.

When you're ready, work through:

- [Deployment walkthrough](/self-hosting/deploy/) — the full server-side setup, from `docker-compose.yml` through HTTPS, auth, and upgrades.
- [Sample deployments](/self-hosting/examples/) — annotated configs for Fly.io and Linode that power `code.confabulous.dev` and `code-demo.confabulous.dev`.
- [Configuration reference](/self-hosting/configuration/) — every environment variable.

## 3. Onboard your team

Once your instance is up and reachable, share its URL with your users and point them at the [Quickstart for end users](/getting-started/end-user-quickstart/). They sign in, run `confab setup --backend-url <your-instance-url>`, and they're done.

## 4. Explore usage

As an admin you can browse every session on the instance without asking anyone to share it: turn on **All sessions** on the Sessions page and open any session. The view is read-only and redacted like a shared session. See [Admin access](/features/sharing/#admin-access).

For an instance-wide view of who uses what, open **Admin → Session Insights**. It covers every listable session on the instance, whoever owns it and whether or not it is shared:

- **Provider share** stacks session counts per provider over time, with a count or percent toggle and a totals table. Buckets are UTC days for ranges up to 31 days, weeks up to a year, and months beyond.
- **Session size distribution** plots one point per session by start time on a log scale. Pick the size measure: transcript lines, tokens (input + output), estimated cost, user turns, or duration. Hover a point for its title, owner, provider, repository, and every measure; click it to open the session. Above 5,000 sessions the chart shows a stable sample and says so.

Filter both charts by date range (7d, 30d, 90d, 1y, all), provider, and owner.
