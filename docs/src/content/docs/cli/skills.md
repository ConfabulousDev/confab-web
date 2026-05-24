---
title: Skills
description: Bundled slash commands installed by confab setup — /til and /retro.
---

`confab setup` ships two **bundled skills** — slash commands that get installed into your provider CLIs (Claude Code, Codex) and that you invoke from inside an active session. Both are pre-wired to talk to the backend you authed against during setup, so there's no extra config.

## How skills are installed

When you run `confab setup`, the CLI:

1. Auto-detects which provider CLIs (`claude`, `codex`) are on your `PATH`.
2. For each detected provider, writes the skill's `SKILL.md` into the provider's skills directory:
   - Claude Code: `~/.claude/skills/<skill>/SKILL.md`
   - Codex: `~/.codex/skills/<skill>/SKILL.md`
3. If you'd previously customized the skill, the existing file is backed up to `SKILL.md.bak` before being overwritten — so re-running setup never silently drops your edits.

You can install or remove skills manually with `confab skills add` / `confab skills remove` (see [Commands](/cli/commands/#skills)).

The bundled skills are:

## `/til` — Today I learned

Captures a short knowledge snippet from the current session and saves it to your Confabulous backend, where it surfaces in the [TILs feed](/features/tils/).

### Usage

Inside an active Claude Code or Codex session:

```
/til <what you learned>
```

The agent:

1. Uses your message as the TIL title.
2. Writes a 2–3 sentence summary that captures what was learned and why it matters, drawing on the conversation context.
3. Invokes `confab til` under the hood to persist the TIL to the backend.
4. Briefly confirms back to you that the TIL was saved.

The TIL deep-links back to the exact message in the originating session, so future-you can click through to the full context.

### Under the hood

The skill is allowed to invoke `confab til *`. Behind the scenes:

```bash
# Claude Code:
confab til --session "${CLAUDE_SESSION_ID}" --title "<title>" --summary "<summary>"

# Codex:
confab til --provider codex --session "${CODEX_THREAD_ID}" --title "<title>" --summary "<summary>"
```

For Codex, the CLI normalizes subagent thread IDs to the root thread before saving — so TILs from a subagent attach to the parent session.

## `/retro` — Session retrospective

Fetches and discusses a session you (or a teammate) ran earlier. Useful for reviewing what happened, extracting learnings, or critiquing the approach.

### Usage

Inside a fresh Claude Code or Codex session:

```
/retro <session-id> [optional question or focus]
```

`<session-id>` is the ID shown on your Confabulous dashboard. The optional trailing text steers the discussion.

The agent:

1. Calls `confab retro` to fetch the condensed transcript plus structured metadata.
2. Writes the JSON and XML output to a timestamped directory under `/tmp/retro-<timestamp>/`.
3. Searches for the local raw transcript file (richer than the backend's condensed view) and reads relevant sections for deeper analysis.
4. Summarizes the session conversationally — what happened, key outcomes, cost, duration, model.
5. If you supplied a question or focus area, answers it; otherwise opens up to general discussion.

### Under the hood

```bash
RETRO_DIR="/tmp/retro-$(date +%s)"
confab retro --output-dir "$RETRO_DIR" <session-id>
```

This writes two files (`response.json` and `transcript.xml`) into the output directory. The skill's prompt is in `SKILL.md` and is what teaches the agent how to use them.

## Customizing or replacing skills

The `SKILL.md` files are plain markdown — you can edit them in place. Your edits are preserved across `confab setup` runs (a `.bak` is written before overwrite).

To pin a customized version: edit `SKILL.md`, then run `confab status` to confirm the skill is still recognized as installed. To revert: delete the file and run `confab skills add`.

To author entirely new skills for your team's workflow, drop a `SKILL.md` into the provider's skills directory directly — Confabulous doesn't restrict what skills can live there beyond its own bundled set.
