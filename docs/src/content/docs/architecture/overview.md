---
title: Architecture overview
description: How the Confabulous backend and frontend fit together.
---

<img src="/how-it-works.svg" alt="Architecture diagram" />

## Components

- **Backend (Go)** — HTTP API, analytics compute, smart recap generation, session storage.
- **Frontend (React/TypeScript)** — single-page app served from the same container as the backend.
- **PostgreSQL** — sessions, users, shares, analytics card data, search index.
- **MinIO** (or S3-compatible) — object storage for transcripts (one chunked blob per session).
- **CLI** ([separate repo](https://github.com/ConfabulousDev/confab)) — file watcher that streams session transcripts to the backend in real time.

## Provider abstraction

Both the parser and the analytics pipeline are organized around a `SessionProvider` interface, with one implementation per provider (Claude Code, Codex). Adding a third provider is documented in `backend/internal/analytics/PROVIDER_EXTENSION.md`.

## Session visibility

A single CTE (`VisibleSessionsCTE`) is the source of truth for "which sessions can this user see." Every list/filter/aggregate endpoint routes through it.
