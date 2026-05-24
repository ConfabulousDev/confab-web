---
title: Introduction
description: What Confabulous is, who it's for, and what you can do with it.
---

Confabulous is an **open-source platform** for archiving, searching, and analyzing your Claude Code and OpenAI Codex sessions. Use the free managed instance at [confabulous.dev](https://confabulous.dev), or self-host the whole stack on your own infrastructure.

## What you get

- A **dashboard** showing every session you've ever run, with full transcripts and analytics.
- **Cost tracking** across providers, models, and time.
- **Smart Recaps** — AI-generated summaries of what each session accomplished.
- **TILs** — Today-I-Learned snippets surfaced from your sessions.
- **Trends** — usage and cost trends for your own sessions over time.
- **Organization Analytics** — per-user aggregated cost and usage across the whole team, for trusted-team deployments.
- **Sharing** — fine-grained per-session sharing, or open policies for high-trust deployments.

## Who it's for

- **Individual developers** who want to keep an archive of their AI coding sessions and track costs.
- **Teams** who want to share interesting sessions, surface learnings, and understand where AI is being used.
- **Organizations** with compliance or privacy requirements that rule out third-party SaaS dashboards.

## How it works

The CLI watches your local Claude Code and Codex session transcripts and streams them to your backend in real time, chunk by chunk, as each session progresses. The backend parses, analyzes, and stores them; the web UI is a thin shell over that data.

See [Concepts](/getting-started/concepts/) for a deeper model of the moving parts.
