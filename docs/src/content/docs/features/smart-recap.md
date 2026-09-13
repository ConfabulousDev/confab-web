---
title: Smart recap
description: AI-generated summaries of what each session accomplished.
---

A **Smart Recap** is a concise AI-generated summary of a session — what was attempted, what worked, what didn't.

## Requirements

Smart Recaps require an Anthropic or OpenAI API key configured on the backend. Without one, the recap card is hidden and sessions show only structural analytics. The recap card shows which provider and model generated it.

## Generation

Recaps are generated on first session view and cached. You can manually regenerate a recap from the session view if the model has improved or the input changed.

## Configuration

Set `ANTHROPIC_API_KEY` in your environment, or set `SMART_RECAP_LLM_PROVIDER=openai` and `OPENAI_API_KEY` to use OpenAI. See [Configuration](/self-hosting/configuration/) for details.
