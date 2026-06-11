---
title: Trends
description: Aggregate usage, cost, and activity trends across many sessions.
---

The **Trends** page aggregates analytics across every session you can see, broken down by provider, model, repo, and time.

## What it shows

- Daily / weekly / monthly cost trends.
- Token usage trends (input, output, cache).
- **Cost by model** — a per-model breakdown of spend and tokens across your sessions, with each model attributed to its provider. Fast-mode usage appears as its own line, and models not yet in the pricing table roll up under "Unknown". This covers sessions that have per-model data; a caption notes the coverage.
- Tool usage and frequency.
- Repo activity.

## Filters

Trends supports a first-class **Owner** filter, a **Model** filter (narrows to sessions that used a chosen model — combine it with the provider filter to scope further), and the standard repo / provider / date-range filters from the Sessions page.

When a model filter is active, the other cost cards (the total-cost headline and Costliest Sessions) show a small ⓘ note: the filter selects whole sessions that used that model, so those figures still reflect each session's full cost rather than only the selected model's share.

## Demo users

The demo identity sees trends across all visible sessions (the demo viewer owns nothing, so the Owner filter is skipped on their behalf).
