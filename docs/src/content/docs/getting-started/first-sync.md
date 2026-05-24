---
title: First sync
description: What to expect the first time the CLI streams sessions to your Confabulous server.
---

After running `confab setup`, the CLI watches your local Claude Code and Codex session transcripts and streams them to your backend in real time as each session progresses.

## What gets uploaded

- The full session transcript (JSONL), streamed chunk by chunk as the session runs.
- Git metadata captured at session start (repo URL, branch, remotes).
- Per-message token counts and model identifiers.

## What does *not* get uploaded

- Any files on disk that weren't shared with the agent.
- Environment variables or secrets from your shell.
- Anything outside the session transcript file.

## Verifying

Open your Confabulous dashboard — your active session should appear within a few seconds of starting, and continue to update as new turns happen. The dashboard URL depends on how Confabulous was deployed for you: the managed instance at [confabulous.dev](https://confabulous.dev), your team's self-hosted URL, or — when you're prototyping the stack on your own laptop — [http://localhost:8080](http://localhost:8080).
