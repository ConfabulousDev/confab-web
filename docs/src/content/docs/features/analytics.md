---
title: Per-session analytics
description: Cost, tokens, duration, and tool usage for a single session.
---

Every session in Confabulous has an analytics summary surfaced alongside the transcript. The summary panel renders one card per dimension; together they answer "what did this session cost, how long did it take, and what did it do?"

See it live on the [demo instance](https://demo.confabulous.dev/sessions/e8b54496-44f2-40f5-94c1-d786b5443901).

## What it shows

- **Cost** — estimated USD cost, computed from token usage and current model pricing.
- **Tokens** — input, output, and cache token totals, broken down by model.
- **Duration** — wall-clock duration, split into assistant response time and user active time.
- **Tools** — every tool the agent invoked, with call counts.
- **Conversation shape** — message count by role (user / assistant), plus a timeline visualization of when each message arrived.

## How it's computed

Analytics are derived from the transcript at sync time. No additional LLM calls are made — the numbers come straight from the structured session data the CLI uploads.

Model pricing tables live in the backend (`backend/internal/analytics/pricing.go`) and are kept in sync with the frontend display table (`src/utils/tokenStats.ts`) via a parity test. When a new model ships, both tables update together.

## Related

- [Organization analytics](/features/organization-analytics/) rolls the same numbers up across every user in your instance.
- [Smart recap](/features/smart-recap/) layers an LLM-generated narrative summary on top of the structural analytics.
