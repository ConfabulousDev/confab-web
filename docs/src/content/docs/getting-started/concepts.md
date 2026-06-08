---
title: Concepts
description: The core data model and how Confabulous's pieces fit together.
---

## Sessions

A **session** is one continuous conversation with an AI coding agent — every Claude Code, Codex, or OpenCode conversation is one Confabulous session.

## Providers

Confabulous treats Claude Code, Codex, and OpenCode as **providers** — each has its own parser, analytics, and transcript shape, but they share a common UI and aggregate analytics. See the [provider pages](/providers/claude-code/) for what each one captures.

## Smart recap

A **Smart Recap** is an AI-generated summary of a session — what was attempted, what worked, what didn't.

## Trends

**Trends** aggregate your own session analytics over time — useful for understanding personal cost drift, model usage, and activity patterns.

## Organization analytics

**Organization Analytics** rolls cost and usage up across every user in your instance, one row per user. Off by default; opt in via `ENABLE_ORG_ANALYTICS` for trusted-team deployments. See [Organization analytics](/features/organization-analytics/) for the privacy implications.
