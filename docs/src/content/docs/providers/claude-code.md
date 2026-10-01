---
title: Claude Code
description: How Confabulous parses, analyzes, and displays Claude Code sessions.
---

Confabulous has first-class support for [Claude Code](https://claude.com/claude-code) sessions.

## What gets parsed

- Full conversation history (user, assistant, tool calls, tool results).
- Token counts per message (input, output, cache read, cache write).
- Model identifier per message.
- Tool invocations and their arguments.
- File edits and reads.

## Analytics cards

Each Claude Code session produces these cards:

- **Tokens** — input/output/cache breakdown.
- **Cost** — using the [pricing table](https://www.anthropic.com/pricing).
- **Tools** — which tools were called and how often.
- **Conversation** — turn structure, active time, message counts.
- **Repo activity** — files touched, language breakdown.

## Subagents

When a session launches subagents with the Agent tool (or the older Task tool), the transcript shows each launch as a subagent card: its description, type, model, and status. Foreground agents also show duration, tokens, and tool-use count. When a background agent finishes, its completion appears as a "Subagent finished" card.

Select **Open transcript** on a card to read that subagent's own conversation. It opens in a tab under **Transcript**, next to **Main**:

- Subagents launched from the main conversation get a tab each, in launch order.
- Each tab shows the subagent's status: running, completed, failed, or stopped. Hover or focus a tab for its type, model, and duration.
- Select the open subagent's tab for **Go to parent**, which returns to the conversation that launched it at the launch point, and **Copy link to this subagent**.
- **All subagents (n)** counts every subagent in the session and lists the ones launched from the main conversation, with status, type, model, and duration. A subagent that launched others shows how many. If subagents launched more, a note under the list says how many. Long lists get a filter.
- The open subagent is in the URL (`?agent=<id>`), so a copied link opens it. Anyone who can view the session can view its subagents.
- Transcript filters apply to whichever tab is open. Search runs within the open tab.
- A running subagent's tab updates live. If its transcript hasn't synced yet, the tab says so and loads when it arrives.

### Nested subagents

Subagents launched by other subagents (for example, judges that an implementer agent starts) don't get tabs. Open a subagent that launched others and a path row appears under the tabs:

- **Launched here (n)** lists the subagents the open one launched.
- The path shows where you are, such as **Main › Implement step › Judge arrays**. Each nested level is a dropdown of the subagents launched alongside it, so you can switch between siblings. Selecting an earlier level returns to it at the point where it launched the next one.
- While you're inside a nested subagent, the tab of the top-level subagent above it stays outlined.
- The copy-link button at the end of the path copies a link to the open nested subagent. Opening that link labels its tab by id until you reach it through its parent.
- The dropdowns remember what each subagent launched for the rest of your visit, even after you move to another subagent.

Subagents started by workflow runs don't appear in the transcript yet.

## Pricing

Confabulous tracks pricing for every published Claude model. New models are added to the pricing table as Anthropic publishes them.

## Multiple backends

You can point separate Claude Code config dirs at separate Confabulous backends from one machine — for example, a personal `CLAUDE_CONFIG_DIR` that syncs to one backend and a work one that syncs to another. Bind a config dir to a backend at setup:

```bash
confab setup --provider claude-code --config-dir <dir> --backend-url <url>
```

`--config-dir` requires `--provider`. Your default config dir keeps using the backend from your original `confab setup`, so existing single-dir setups are unaffected.

## Other supported providers

Confabulous treats every provider as a first-class citizen. [Codex](/providers/codex/), [Cursor](/providers/cursor/), and [OpenCode](/providers/opencode/) are also supported today. New providers slot into the same sync, storage, and analytics pipeline.
