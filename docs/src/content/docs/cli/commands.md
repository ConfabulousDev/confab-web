---
title: Commands
description: Full subcommand reference for the confab CLI.
---

The `confab` CLI is built on [Cobra](https://github.com/spf13/cobra). Run `confab --help` or `confab <command> --help` at any time for the canonical, locally-installed reference. This page is organized by what you'd actually use each command for.

## Lifecycle

| Command | What it does |
|---------|--------------|
| `confab setup --backend-url <url>` | One-command onboarding: auth + install hooks + install bundled skills for every detected provider. See [Overview](/cli/overview/#setup). |
| `confab status` | Print backend auth state and per-provider hook/skill status. Detects orphan hooks (hook installed but the provider CLI isn't on `PATH`). |
| `confab login --backend-url <url>` | Just the auth step (device-code flow, or `--api-key` to skip). |
| `confab logout` | Clear stored credentials. |
| `confab update` | Check for and install a newer CLI release. |
| `confab autoupdate enable\|disable` | Toggle automatic update checks. |
| `confab version` | Print version, build info, and config dir path. |

## Hooks

Hooks are the bridge between your provider CLI (Claude Code, Codex) and the sync daemon. `confab setup` installs them automatically; these commands are for manual repair.

| Command | What it does |
|---------|--------------|
| `confab hooks add --provider <name>` | Install Confabulous hooks into the provider CLI's config. |
| `confab hooks remove --provider <name>` | Remove Confabulous hooks from the provider CLI's config. |

The individual `confab hook session-start` / `session-end` / `pre-tool-use` / `post-tool-use` / `user-prompt-submit` subcommands are invoked **by the provider CLI**, not by you directly. They read JSON from stdin and write JSON to stdout — running them by hand isn't useful.

## Skills

| Command | What it does |
|---------|--------------|
| `confab skills add` | Install the bundled `/til` and `/retro` skills for detected providers (also done automatically by `confab setup`). |
| `confab skills remove` | Remove the bundled skills. |

See [Skills](/cli/skills/) for what `/til` and `/retro` actually do.

## Sync daemon

The sync daemon is the long-running process that streams transcript chunks to the backend. You usually don't touch it — it spawns and exits with each session. These commands are for troubleshooting.

| Command | What it does |
|---------|--------------|
| `confab sync status` | Show which sync daemons are running, per-session. |
| `confab sync start` | Start a daemon for the current session (rarely needed — hooks do this). |
| `confab sync stop` | Stop the daemon for the current session. |

## Sessions

| Command | What it does |
|---------|--------------|
| `confab list` | List local sessions discovered on disk (Claude Code and Codex). Supports duration and provider filters. |
| `confab save <session-id> [--provider X]` | Manually upload a specific session by its provider-internal ID. Useful for backfilling sessions that ran before hooks were installed. For Codex, `save` performs the same tree walk-up as the hook, so passing any subagent UUID syncs the whole tree. |
| `confab session get-summary <id>` | Fetch a condensed transcript for an uploaded session from the backend. |
| `confab session download <id>` | Download the raw JSONL transcript files for an uploaded session. |
| `confab session list-files <id>` | List transcript-file metadata for an uploaded session. |

## TIL and retro

| Command | What it does |
|---------|--------------|
| `confab til --session <id> --title <t> --summary <s>` | Save a TIL ("Today I Learned") to the backend. Normally invoked by the `/til` skill; see [Skills](/cli/skills/). |
| `confab retro <session-id> [--output-dir <dir>]` | Fetch the condensed transcript plus structured metadata for retrospective analysis. Normally invoked by the `/retro` skill. |

## Utilities

| Command | What it does |
|---------|--------------|
| `confab install` | Copy the running `confab` binary into `~/.local/bin/` (manual alternative to the install script). |
| `confab redaction-test <file>` | Test your current redaction rules against a sample transcript file before they affect real uploads. |
| `confab announce` | Internal — used by the auto-update flow to surface release-notes messages. |

## Flags shared across commands

- `--backend-url` — passed to commands that hit the backend (`setup`, `login`, `save`, `til`, `retro`, `session ...`). When omitted, the CLI uses the URL stored in `~/.confab/config.json` from the last `setup` / `login`.
- `--api-key` — bypass the device-login flow for `setup` / `login`.
- `--provider <claude-code|codex>` — disambiguate which provider a command targets. Defaults vary per command; `confab list`, `save`, and `til` route through the provider interface so adding a new provider doesn't require changes here.
