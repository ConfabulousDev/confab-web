---
title: Sharing
description: Share individual sessions with teammates, or open the floodgates for high-trust deployments.
---

Confabulous supports two sharing modes:

## Per-session sharing (default)

You explicitly share a session with another user by email. Sharing is recorded in the database; the recipient sees the session in their **Shared with me** view.

## Open sharing policy

For high-trust deployments (e.g. a small team), you can flip an instance-wide config flag (`SHARE_ALL_SESSIONS`) so every user sees every session. Personal explicit shares still win the priority dedup, so workflows that depend on the recipient column keep working.

## Visibility model

The session visibility predicate is the single source of truth across the dashboard, Sessions API, TILs API, and Trends. See [Architecture overview](/architecture/overview/).
