---
title: Demo mode
description: Configure a read-only demo identity for showcasing your Confabulous instance.
---

Confabulous supports a **demo identity** — a read-only, shared-cookie user account that anyone can log in as without credentials. Useful for showcasing the product (this is what powers [demo.confabulous.dev](https://demo.confabulous.dev)).

## Enabling

Set a single environment variable:

```bash
DEMO_IDENTITY_EMAIL=demo@example.com
```

When set, the demo user can sign in by visiting the login page — no password required. Their session is automatically impersonated, and write operations return a friendly "read-only" toast in the UI.

When unset, every demo-mode predicate short-circuits to today's behavior — there's zero overhead.

## What demo users can and cannot do

| Action | Demo user |
| --- | --- |
| View their own sessions | ✅ |
| View shared sessions | ✅ |
| View public Trends | ✅ |
| Upload new sessions | ❌ |
| Share or unshare sessions | ❌ |
| Change account settings | ❌ |

## Security notes

The demo cookie is HMAC-derived and shared across all visitors of the demo identity. The implementation enforces read-only at the middleware layer, not just the UI — see `backend/internal/auth/demo.go` for the canonical reference.

## Reference deployment

[`ConfabulousDev/confab-demo-site`](https://github.com/ConfabulousDev/confab-demo-site) is the live stack behind [demo.confabulous.dev](https://demo.confabulous.dev). The combination that makes the site anonymous-readable and sign-ups-closed, from [`stack/docker-compose.yml`](https://github.com/ConfabulousDev/confab-demo-site/blob/main/stack/docker-compose.yml):

```yaml
DEMO_IDENTITY_EMAIL: demo@confabulous.dev
MAX_USERS: "0"
ALLOWED_EMAIL_DOMAINS: confabulous.dev
SHARE_ALL_SESSIONS_TO_AUTHENTICATED: "true"
ENABLE_SHARE_CREATION: "false"
AUTH_PASSWORD_ENABLED: "true"  # operator signs in to upload; OAuth not configured
```
