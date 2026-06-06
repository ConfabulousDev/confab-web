---
title: Codex
description: How Confabulous parses, analyzes, and displays OpenAI Codex sessions.
---

Confabulous has first-class support for [OpenAI Codex](https://developers.openai.com/codex) sessions, including subagent spawns and skill invocations.

## What gets parsed

- Full conversation history.
- Per-message token counts (input, output, cached input, reasoning).
- Model identifier (gpt-5, gpt-5.5, o3, etc.).
- Tool calls.
- **Subagent spawns** (`spawn_agent` / `wait_agent`) — bucketed by agent role.
- **Skill invocations** (`<skill>` user-message wrappers) — bucketed by skill name.
- Parent-child thread relationships (recursive tree of spawned subagents).

## Analytics cards

- **Tokens** — including reasoning tokens (preserved for display; billed at output rate).
- **Cost** — using the [OpenAI pricing table](https://developers.openai.com/api/docs/pricing).
- **Tools, Agents & Skills** — Codex-specific breakdown.
- **Conversation** — Codex synthesizes reasoning time into active time.
- **Repo activity**.

## Subagent aggregation

When a Codex session spawns subagents, Confabulous aggregates the main thread plus every subagent thread for most analytics cards. The Conversation card stays main-only by design.

## Pricing nuances

- `cached_input_tokens` is a subset of `input_tokens` (not a separate count).
- `reasoning_output_tokens` is a subset of `output_tokens` (billed at output rate).
- OpenAI does not charge for cache writes.

## Other supported providers

Confabulous treats every provider as a first-class citizen. [Claude Code](/providers/claude-code/) and [OpenCode](/providers/opencode/) are also supported today. New providers slot into the same sync, storage, and analytics pipeline.
