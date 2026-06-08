---
title: PR linking
description: Two-way links between Confabulous sessions and the GitHub PRs and commits they produced.
---

Confabulous links each session to the pull requests and commits the agent produced during that session, and the inverse direction is also wired up: every linked PR and commit carries a reference back to its originating session.

:::note
GitHub commit and PR linking is available for Claude Code and Codex sessions. OpenCode sessions are synced and analyzed, but do not produce these links.
:::

## Confabulous → GitHub

Every session that touches a git repo records the commits it created and any PRs those commits land in. The session detail view surfaces these as a chip row above the transcript: each chip is a clickable link to the PR or commit on GitHub.

The linkage is derived at sync time from the structured git events the agent emits — no per-PR GitHub API call is needed at view time.

## GitHub → Confabulous

The reverse direction works through PR descriptions and commit messages. When the agent opens a PR or makes a commit during a Confabulous session, it includes a link back to the originating session in the PR body and the commit footer. Anyone reading the PR on GitHub can jump straight to the full transcript that produced it.

This makes session deep-links durable: even months later, looking at a merged PR, the surrounding context is one click away.

## Fork resolution

When a session touches a fork, Confabulous resolves the fork to its upstream root for the purposes of repo filtering and PR linking. A PR opened against `upstream/main` from a fork branch still shows up as belonging to `upstream`, not the fork.

## Related

- [Sessions](/features/sessions/) — the session viewer where the PR chips render.
- [Architecture overview](/architecture/overview/) — how session data is synced and stored.
