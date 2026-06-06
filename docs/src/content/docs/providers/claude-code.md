---
title: Claude Code
description: How Confabulous parses, analyzes, and displays Claude Code sessions.
---

Confabulous has first-class support for [Claude Code](https://claude.com/claude-code) sessions.

## What gets parsed

- Full conversation history (user, assistant, tool calls, tool results).
- Token counts per message (input, output, cache read, cache write).
- Model identifier per message.
- Tool invocations and their arguments.
- File edits and reads.

## Analytics cards

Each Claude Code session produces these cards:

- **Tokens** — input/output/cache breakdown.
- **Cost** — using the [pricing table](https://www.anthropic.com/pricing).
- **Tools** — which tools were called and how often.
- **Conversation** — turn structure, active time, message counts.
- **Repo activity** — files touched, language breakdown.

## Pricing

Confabulous tracks pricing for every published Claude model. New models are added to the pricing table as Anthropic publishes them.

## Other supported providers

Confabulous treats every provider as a first-class citizen. [Codex](/providers/codex/) and [OpenCode](/providers/opencode/) are also supported today. New providers slot into the same sync, storage, and analytics pipeline.
