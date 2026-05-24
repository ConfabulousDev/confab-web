---
title: Quickstart for end users
description: Sign in to your Confabulous instance and connect the CLI to start syncing sessions.
---

If your team or organization already runs a Confabulous instance — or you're using the managed instance at [confabulous.dev](https://confabulous.dev) — this is the only page you need.

Setting up the instance itself is a separate job; see the [Quickstart for admins](/getting-started/admin-quickstart/) if you're the one running it.

## 1. Sign in to the web dashboard

Open your team's Confabulous URL in a browser and sign in. The URL depends on how your team has Confabulous set up:

- **Managed instance:** [confabulous.dev](https://confabulous.dev)
- **Self-hosted instance:** ask your admin — typically something like `https://confab.your-company.com`

## 2. Install the CLI

```bash
curl -fsSL https://raw.githubusercontent.com/ConfabulousDev/confab/main/install.sh | bash
```

## 3. Connect the CLI to your instance

Run `confab setup` and pass the same backend URL you used to sign in:

```bash
confab setup --backend-url https://confabulous.dev
```

Or for a self-hosted instance:

```bash
confab setup --backend-url https://confab.your-company.com
```

The setup flow opens a browser tab to authorize the CLI against your account, then stores an API token in `~/.confab/` and installs sync hooks plus the `/til` and `/retro` skills for every supported provider CLI (`claude`, `codex`) detected on your `PATH`.

## 4. Start a session

Start any Claude Code or Codex session. The CLI streams transcripts to your backend in real time — open your dashboard and you'll see the session appear and update live as you work.

## What's next

- [First sync](/getting-started/first-sync/) — what to expect on your first uploads.
- [CLI overview](/cli/overview/) — subcommands, config, and troubleshooting.
- [Concepts](/getting-started/concepts/) — sessions, providers, TILs, recaps.
