---
title: CLI overview
description: The confab CLI that syncs your Claude Code and Codex sessions in real time.
---

The `confab` CLI is the bridge between your local AI coding sessions and the Confabulous backend. It lives in a separate repository: [ConfabulousDev/confab](https://github.com/ConfabulousDev/confab).

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/ConfabulousDev/confab/main/install.sh | bash
```

The install script fetches a pre-built binary from [GitHub Releases](https://github.com/ConfabulousDev/confab/releases) for your OS and architecture and drops it on your `PATH`. If you'd rather build from source, clone the repo and `go install ./...`.

## Setup

```bash
confab setup --backend-url https://your-confab-instance.example.com
```

`--backend-url` is required — the CLI doesn't have a default backend. Pass the URL of the Confabulous instance you sign in to in the browser:

- **Managed:** `--backend-url https://confabulous.dev`
- **Self-hosted:** `--backend-url https://confab.your-company.com`
- **Local prototype:** `--backend-url http://localhost:8080`

The setup flow:

1. **Authenticates** against the backend via a browser device-login flow. Pass `--api-key cfb_...` to skip the device flow and use a key directly.
2. **Auto-detects** which provider CLIs (`claude`, `codex`) are installed on your `PATH`.
3. **Installs the sync hooks** into each detected CLI's configuration.
4. **Installs the bundled skills** (`/til` and `/retro`) for each detected CLI.

Pass `--provider claude-code` or `--provider codex` to restrict setup to one provider.

To verify everything landed correctly, run `confab status` — it prints the backend URL, auth state, and per-provider hook/skill state, with remediation hints if anything's off.

## How sync works

Sync is driven by the **hooks** `confab setup` installs into each provider CLI's configuration. When Claude Code or Codex runs a session, it invokes these hooks at key lifecycle points (session start, user prompt submit, tool use, session end) — and the hooks stream new transcript chunks to your backend in real time, chunk by chunk. You don't have to wait for the session to end before it appears on the dashboard: new messages, tool calls, and analytics surface as they happen.

A persistent sync **daemon** is spawned per session to handle the streaming. You don't manage it directly — it starts on `session-start` and exits when the session ends — but you can inspect or restart it with `confab sync status` / `confab sync start` if needed.

All upload traffic goes over the [HTTP API](/api/overview/) authenticated with your API token.

## Config directory: `~/.confab/`

All CLI state lives under `~/.confab/`:

| Path | Contents |
|------|----------|
| `~/.confab/config.json` | API key, backend URL, redaction config |
| `~/.confab/state/` | Per-session daemon state (one file per active session) |
| `~/.confab/inbox/` | Queued uploads pending the next sync flush |
| `~/.confab/logs/` | CLI and daemon logs |
| `~/.confab/update.json` | Auto-update timestamp tracking |

The directory is created on first `confab setup`. Deleting it forces a clean re-auth on the next setup; deleting `state/` while a session is running may leave orphaned daemons, so prefer `confab logout` + `confab setup` for a reset.

## Next steps

- [Commands](/cli/commands/) — full subcommand reference.
- [Skills](/cli/skills/) — the bundled `/til` and `/retro` slash commands.
