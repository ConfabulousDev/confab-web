---
title: Sharing
description: Share individual sessions with teammates, or open the floodgates for high-trust deployments.
---

Confabulous Code supports two sharing modes:

## Per-session sharing (default)

You explicitly share a session with another user by email. Sharing is recorded in the database; the recipient sees the session in their **Shared with me** view.

## Open sharing policy

For high-trust deployments (e.g. a small team), you can flip an instance-wide config flag (`SHARE_ALL_SESSIONS`) so every user sees every session. Personal explicit shares still win the priority dedup, so workflows that depend on the recipient column keep working.

## Admin access

Admins (anyone in `SUPER_ADMIN_EMAILS` or granted admin in the admin UI) can open any session on the instance and list every session with the **All sessions** toggle on the Sessions page. Nobody else gains access: no share is created. Admin access is always on; there is no config flag to turn it off.

An admin sees the same redacted view as a share recipient: no working directory, hostname, OS username, transcript path, or git remote URLs, plus the owner's email. Admins can't edit, share, or delete sessions they don't own. A banner reading **Viewing as admin** marks these views. Admins can also open sessions whose owner has been deactivated. Admin views aren't logged.

So the people who can see a session are its owner, the people it's shared with (or everyone, under the open sharing policy), and admins.

## Visibility model

The session visibility predicate is the single source of truth across the dashboard, Sessions API, and Trends. See [Architecture overview](/architecture/overview/).
