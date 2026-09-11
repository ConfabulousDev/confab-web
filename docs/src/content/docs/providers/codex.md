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
- **Code activity** — files modified and lines added/removed, from whichever edit format your Codex CLI version emits.
- **Repo activity**.

## Codex CLI version differences

Codex reshaped its rollout format in version 0.149.1. Confabulous reads both formats, so old and new sessions analyze correctly, but two things legitimately differ between them.

**Tool names are reported as Codex recorded them.** Sessions from 0.130.0 and earlier show `exec_command`, `apply_patch` and `write_stdin`; sessions from 0.149.1 onward show `exec`, `wait` and `web.search`. Codex renamed its tools, so a session from before the change and one from after will list different tool names for the same kind of work. Confabulous does not remap them onto a common vocabulary — that would mean displaying names that never appeared in your transcript.

**Files read and searches are only available from 0.149.1 onward.** Newer Codex versions record what each shell command was doing, so Confabulous can count file reads and searches. Older versions recorded only the command line itself, with no indication of its purpose, so those two figures stay at zero for older sessions rather than being guessed at from command text.

## Subagent aggregation

When a Codex session spawns subagents, Confabulous aggregates the main thread plus every subagent thread for most analytics cards. The Conversation card stays main-only by design.

## Pricing nuances

- `cached_input_tokens` is a subset of `input_tokens` (not a separate count).
- `reasoning_output_tokens` is a subset of `output_tokens` (billed at output rate).
- OpenAI does not charge for cache writes.

## Other supported providers

Confabulous treats every provider as a first-class citizen. [Claude Code](/providers/claude-code/), [Cursor](/providers/cursor/), and [OpenCode](/providers/opencode/) are also supported today. New providers slot into the same sync, storage, and analytics pipeline.
