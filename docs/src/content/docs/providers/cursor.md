---
title: Cursor
description: How Confabulous parses, analyzes, and displays Cursor agent sessions.
---

Confabulous has first-class support for [Cursor](https://cursor.com) agent sessions. Cursor records a slimmer transcript than other providers, so a few cards behave differently — this page describes exactly what Confabulous reads and what it cannot.

## What gets parsed

- Conversation history: user and assistant text from the main agent thread.
- Tool **calls** — the tool name plus a one-line summary of its input.
- File reads and edits, inferred from those tool calls.

Cursor's agent transcript records tool **inputs only**, so there are no tool **outputs** to display. Confabulous shows each tool call and its arguments, but not the result the tool returned.

Cursor hides some scaffolding in its own UI and writes a bare `[REDACTED]` placeholder into the transcript instead; Confabulous strips that on-disk placeholder so transcripts read cleanly. This is Cursor's own scaffolding, not a redacted secret — it is unrelated to the `[REDACTED:TYPE]` markers the confab CLI writes when it scrubs secrets on other providers.

## Analytics cards

Each Cursor session produces the same provider-agnostic cards as every other provider:

- **Tools** — which tools were called and how often, using Cursor's own tool names (`Read`, `Grep`, `Glob`, `SemanticSearch`, `StrReplace`, `Write`, `Shell`, `Delete`, `WebSearch`, `AskQuestion`, `Task`).
- **Conversation** — turn structure, active time, and message counts.
- **Code activity** — files touched and language breakdown, derived from Cursor's file tool calls.
- **Agents & skills** — present for consistency; see [Subagents](#subagents) below.
- **Session** — high-level session metadata.

There is no **Tokens** or **Cost** card for Cursor — see [Tokens and cost](#tokens-and-cost).

## Tokens and cost

**Per-session token counts and cost are not available for Cursor sessions.** This is a limitation of the data Cursor stores locally, not a gap Confabulous can backfill:

- Cursor meters tokens server-side. The local agent transcript Confabulous ingests has no usable token counts — Cursor staff have confirmed the local `tokenCount` value is best-effort and unreliable.
- Cursor billing is subscription and usage-pool based, so there is no per-message dollar figure to attribute to a session.

Instead of hiding the Tokens card or showing misleading zeros, Confabulous shows an info callout on the Session Summary tab and **"Not available"** placeholders (with tooltips) on token and cost rows. The session list cost column shows an em dash rather than `$0.00`. On Trends, when Cursor sessions are in the selected window, the Tokens & Cost card shows the same unavailable state for Cursor's section and an ⓘ caveat on the card title.

Real Cursor cost analytics, sourced from Cursor's Dashboard / Admin API, are planned as future work — they are not part of this release.

## Model

Cursor's agent transcript does not record a per-message model identifier, so Confabulous does not show a model for Cursor sessions yet. Model attribution is planned for a future release.

## Subagents

Cursor can spawn subagents (stored as separate transcript files), but in this release Confabulous processes the **main thread only**. Subagent threads are not fetched or aggregated yet. The Agents & skills card therefore reflects the main thread alone.

## Deep links

Cursor's transcript has no stable per-message or per-tool identifiers, so deep-linking directly to a specific Cursor message is not supported. Within the app the transcript pane renders normally; it just can't be anchored to an individual message the way other providers can.

## Other supported providers

Confabulous treats every provider as a first-class citizen. [Claude Code](/providers/claude-code/), [Codex](/providers/codex/), and [OpenCode](/providers/opencode/) are also supported today. New providers slot into the same sync, storage, and analytics pipeline.
