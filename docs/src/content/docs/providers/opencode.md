---
title: OpenCode
description: How Confabulous parses, analyzes, and displays OpenCode sessions.
---

Confabulous has first-class support for [OpenCode](https://opencode.ai) sessions.

## What gets parsed

- Full conversation history (user, assistant, tool calls, tool results).
- Per-message token counts (input, output, cache read, cache write, reasoning).
- Model identifier per message — OpenCode can run models from any of its supported providers.
- Tool invocations and their arguments.
- File edits and reads.

## Analytics cards

Each OpenCode session produces these cards:

- **Tokens** — input/output/cache breakdown.
- **Cost** — based on the model's published pricing.
- **Tools** — which tools were called and how often.
- **Conversation** — turn structure, active time, message counts.
- **Repo activity** — files touched, language breakdown.

## Pricing

Because OpenCode runs models from many providers, each session is priced against the published rates for whichever model produced it.

## Other supported providers

Confabulous treats every provider as a first-class citizen. [Claude Code](/providers/claude-code/) and [Codex](/providers/codex/) are also supported today. New providers slot into the same sync, storage, and analytics pipeline.
