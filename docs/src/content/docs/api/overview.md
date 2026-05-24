---
title: API reference
description: HTTP API surface for the Confabulous backend.
---

The Confabulous backend exposes a JSON HTTP API used by the web frontend, the [`confab` CLI](https://github.com/ConfabulousDev/confab), and external integrations. All endpoints are prefixed with `/api/v1` unless otherwise noted.

:::note
The canonical reference is maintained alongside the source code at [`backend/API.md`](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md). This page is a structured index — follow the links for the complete request/response schemas.
:::

## Authentication

Two methods:

- **API key** (Bearer token, used by the CLI) — `Authorization: Bearer cfb_...`.
- **Session cookie** (used by the web UI) — `confab_session` cookie, set after OAuth or password login. CSRF is enforced via Fetch metadata validation; no token required.

[Full authentication reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#authentication)

## Endpoint groups

### CLI endpoints (API key auth)

The bulk of the upload pipeline. Used by the `confab` CLI to register sessions and stream transcript chunks in real time as sessions progress.

- `GET /api/v1/auth/validate` — validate an API key.
- `POST /api/v1/sync/init` — start or resume a sync session.
- `POST /api/v1/sync/chunk` — upload a transcript chunk.
- `POST /api/v1/sync/complete` — finalize an upload.

[Full CLI endpoint reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#cli-endpoints-api-key-auth)

### External API endpoints (API key auth)

For programmatic integrations — third-party tools that want to query sessions or analytics.

[Full external API reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#external-api-endpoints-api-key-auth)

### Web endpoints (session auth)

Everything the dashboard talks to: list sessions, fetch session detail, regenerate smart recap, manage shares, trends, TILs.

[Full web endpoint reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#web-endpoints-session-auth)

### OAuth endpoints

OAuth callback URLs for GitHub, Google, and generic OIDC providers.

[Full OAuth reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#oauth-endpoints-no-prefix)

### Admin endpoints (super-admin only)

User management, activation, storage monitoring.

[Full admin reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#admin-endpoints-super-admin-only)

### Public endpoints (no auth)

Shared-session view, health checks, public assets.

[Full public reference →](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#public-api-endpoints-no-auth)

## Conventions

- [Error responses](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#error-responses) — uniform error envelope.
- [Rate limits](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#rate-limits) — per-user and per-endpoint.
- [Request body size limits](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#request-body-size-limits).
- [Email domain restrictions](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#email-domain-restrictions) — `ALLOWED_EMAIL_DOMAINS` enforcement.
- [Read-only identity (Demo Mode)](https://github.com/ConfabulousDev/confab-web/blob/main/backend/API.md#read-only-identity-cf-483-demo-mode) — how mutating requests from the demo user are blocked.
