---
title: Organization analytics
description: Per-user aggregated analytics across every session in your organization.
---

The **Organization Analytics** view rolls up usage and cost across every user in your Confabulous instance. It's designed for trusted-team deployments where the team wants full visibility into how AI coding is being used — who's running sessions, which repos and providers they're hitting, and how much it's costing.

See it live on the demo instance: [demo.confabulous.dev/organization](https://demo.confabulous.dev/organization).

## What it shows

A single page with one row per user and filters across the top:

- **Per-user totals** — session count, total cost, total duration, total assistant time, total user time.
- **Per-user averages** — cost per session, duration per session, assistant/user time per session.
- **Filters** — date range (up to 90 days), provider (Claude Code / Codex), repo.

The repo filter rolls forks up to their upstream root automatically, matching the behavior on Sessions and Trends.

## Enabling it

Set a single environment variable on the `app` service:

```yaml
ENABLE_ORG_ANALYTICS: "true"
```

Restart the stack. The Organization page appears in the top navigation for every authenticated user; the API endpoints `GET /api/v1/org/analytics` and `GET /api/v1/org/repos` start returning data instead of `404`.

When `ENABLE_ORG_ANALYTICS` is unset (or any value other than `"true"`), the feature is fully off — the UI nav item is hidden, the endpoints return `404`, and there is no measurable overhead.

## Privacy model

:::caution
Once enabled, **every authenticated user can see every other user's name, email, session count, cost, and time breakdowns**. There is no role-based gating — admins and regular users see the same data.
:::

This is a deliberate design choice for trusted-team deployments. If you need to limit visibility to admins only, **do not enable this feature**.

If [`ALLOWED_EMAIL_DOMAINS`](/self-hosting/configuration/#shared-auth-settings) is configured, access is implicitly scoped to users in the allowed domains (only those users can log in in the first place).

## Recommended pairings

Organization Analytics works well alongside:

- **[`SHARE_ALL_SESSIONS_TO_AUTHENTICATED`](/features/sharing/#open-sharing-policy)** — combine with org analytics so users can drill from a teammate's aggregate numbers into the individual sessions behind them.
- **[Smart Recaps](/features/smart-recap/)** — gives the aggregate rows narrative context once you click through to specific sessions.

## API

See the [API reference](/api/overview/) for the underlying endpoints (`GET /api/v1/org/analytics`, `GET /api/v1/org/repos`).
