# Cursor Integration Architecture

## Overview

Add Cursor as a fourth provider (alongside Claude Code, Codex, and OpenCode).
Cursor's agent writes JSONL transcripts to disk under `~/.cursor`, so the
integration follows the **file-based daemon pattern** already used for Claude
Code and Codex — a CLI daemon watches the transcript files and uploads chunks —
rather than the HTTP/SSE pattern used for OpenCode.

This document is the **wire-format spec and sync contract** the backend parser
(4r41), analytics compute (gevp), and frontend adapter (18n2) build against. It
is paired with the sanitized fixture
`backend/internal/analytics/testdata/cursor/main.jsonl`, which encodes every
line and block shape described below.

> **This is not Claude Code JSONL.** Claude's `TranscriptLine`
> (`backend/internal/analytics/parser.go`) keys on top-level `type` / `uuid` /
> `timestamp` per line. Cursor's conversation rows have **none** of those — they
> use a `{role, message:{content:[…]}}` envelope. A parser that assumes the
> Claude envelope will silently drop every Cursor line. See
> [Why this differs from Claude Code](#why-this-differs-from-claude-code).

## On-disk layout

```
~/.cursor/projects/<sanitized-project-path>/agent-transcripts/
  └── <session-uuid>/
        ├── <session-uuid>.jsonl          # main thread  → file_type=transcript
        └── subagents/
              └── <subagent-uuid>.jsonl   # subagent thread → DEFERRED (out of v1 scope)
```

- `<sanitized-project-path>` is the absolute working-directory path with `/`
  replaced by `-` (e.g. `Users-jackie-dev-confab-web`).
- `<session-uuid>` is a v4 UUID; the directory name and the main file's basename
  are the **same** UUID. This UUID is the canonical Cursor session id.
- The `subagents/` directory exists on disk and uses the identical line envelope
  (plus a subagent-only `UpdateCurrentStep` tool), but **subagent transcripts
  are deferred from v1** — see [Deferred: subagents](#deferred-subagents).

Session metadata lives in a **separate** tree:

```
~/.cursor/chats/<workspace-hash>/<session-uuid>/meta.json
```

See [Session metadata (meta.json)](#session-metadata-metajson).

## Sync contract (v1)

| Source file | Maps to |
|-------------|---------|
| `agent-transcripts/<uuid>/<uuid>.jsonl` (main) | `file_type=transcript` |
| `agent-transcripts/<uuid>/subagents/<uuid>.jsonl` | **deferred** — not synced in v1 |

This mirrors the Codex/OpenCode convention (main file → `transcript`, subagent
files → `agent`). Only the main transcript is in scope for v1; the `agent`
mapping for subagents is reserved for the follow-up ticket.

The Cursor session UUID is used as the upstream session id. Because the JSONL
lines carry no inline timestamps, model, or token usage (see
[Known gaps](#known-gaps)), the CLI supplies what it can as **sync metadata** on
chunk upload (see ticket 4r41):

- `metadata.latest_message_at` — sourced from
  `~/.cursor/chats/<workspace-hash>/<session-uuid>/meta.json` → `updatedAtMs`
  (epoch milliseconds → RFC3339), when that file is present.
- `metadata.model` — best-effort, sourced from Cursor's app-support
  `state.vscdb` (`composerData.modelConfig.modelName`, joined by
  `composerId == <session-uuid>`). See [Known gaps](#known-gaps) for why this is
  best-effort and not derivable from anything under `~/.cursor`.

## Wire format: JSONL line schema

Every line is a single self-contained JSON object. There are exactly **two**
top-level line shapes.

### 1. Conversation rows

```json
{ "role": "user" | "assistant", "message": { "content": [ <block>, … ] } }
```

- The **only** top-level keys are `role` and `message`.
- There is **no** top-level `type`, `uuid`, `id`, `timestamp`, `model`, or
  `usage` on a conversation row.
- `message.content` is a non-empty array of content blocks (below).

### 2. Turn markers

```json
{ "type": "turn_ended", "status": "success" }
{ "type": "turn_ended", "status": "error", "error": "<message>" }
```

- These rows have **no** `role`.
- `status` is `"success"` or `"error"`. `error` (a human-readable string) is
  present only when `status == "error"` (e.g. usage-limit messages).
- `turn_ended` is the **only** non-conversation `type` observed. No
  `session_started`, `metadata`, or other marker rows exist.

## Wire format: content blocks

Blocks inside `message.content[]` have exactly two shapes:

| Block type | Keys | Notes |
|------------|------|-------|
| `text` | `type`, `text` | Plain assistant/user text. |
| `tool_use` | `type`, `name`, `input` | Tool invocation. **No `id`.** |

- **No `tool_result` blocks exist anywhere.** Cursor records tool **inputs**
  only, never tool **outputs** — this is by design (confirmed by the Cursor forum
  and third-party parsers; see [External corroboration](#external-corroboration)).
- **User** messages are always **text-only**.
- Every **assistant** message has **≥1 `text` block**, optionally followed by one
  or more `tool_use` blocks.

### `id` caveat

There is **no** line-, message-, or block-level `id`/`uuid` anywhere in the
JSONL. The only `id` fields that appear are **nested inside `AskQuestion` input**
(question ids and option ids). Downstream code that wants a stable per-tool-call
key must synthesize one (e.g. positional index); it cannot read one from the
wire.

## Main-thread tools

The full set of tool `name` values observed across a complete local scan
(4 main transcripts, Jun 2026), with occurrence counts for realism:

| Tool | Count | `input` keys | Role |
|------|-------|--------------|------|
| `Read` | 60 | `path` (+ `limit`, `offset`) | file read |
| `Shell` | 50 | `command`, `description`, `required_permissions`; occasionally `request_smart_mode_approval`, `smart_mode_block_reason` | shell |
| `Grep` | 27 | `pattern`, `path` and/or `glob`, `output_mode`, `head_limit`, plus flag keys (`-i`, `-A`, …) | search |
| `Write` | 19 | `path`, `contents` | file create |
| `StrReplace` | 14 | `path`, `old_string`, `new_string` | **file EDIT** |
| `Glob` | 8 | `glob_pattern` (+ `target_directory`) | search |
| `Delete` | 6 | `path` | file delete |
| `WebSearch` | 5 | `search_term`, `explanation` | web search |
| `AskQuestion` | 5 | `title`, `questions[]` `{id, prompt, options[{id,label}]}` | user prompt |
| `Task` | 1 | `description`, `prompt`, `readonly`, `subagent_type` | subagent spawn |
| `SemanticSearch` | 1 | `query`, `target_directories[]`, `num_results` | search |

### Two load-bearing facts for the analytics layer

1. **`StrReplace` is Cursor's file-EDIT tool** — the single most common
   mutation. It does **not** share a name with Claude's `Edit`/`MultiEdit`;
   any provider-agnostic code-activity card (gevp) must map `StrReplace` →
   "edit" explicitly. Missing it means under-counting Cursor edits.
2. **The file-path field is `path`** for every file tool (`Read`, `Write`,
   `StrReplace`, `Delete`) — **not** `file_path` (Claude's key) and not
   `filePath` (OpenCode's key). The code-activity card keys on `path` for
   Cursor.

## Session metadata (meta.json)

`~/.cursor/chats/<workspace-hash>/<session-uuid>/meta.json`:

```json
{ "schemaVersion": 1, "createdAtMs": 1718500000000, "updatedAtMs": 1718500900000, "hasConversation": true, "title": "Code Explorer" }
```

| Key | Notes |
|-----|-------|
| `schemaVersion` | Observed `1`. |
| `createdAtMs` | Session creation, epoch ms. |
| `updatedAtMs` | Last update, epoch ms. **Source for `latest_message_at`.** Present in every observed file. |
| `hasConversation` | Boolean. |
| `title` | **Optional** — absent in some sessions. The CLI must not assume it exists. |

There are **no** token, usage, cost, or model fields in `meta.json`.

## Known gaps

These are the most-asked questions; the answers are definitive as of Jun 2026.

### No inline timestamps

JSONL lines carry no per-line timestamp. The only time signal is
`meta.json.updatedAtMs` (session-level, last-update only). The CLI passes it as
`metadata.latest_message_at`; per-message timing is not recoverable.

### No tokens or cost anywhere Confab syncs

- **JSONL:** no `usage`/token/cost fields at all.
- **Local `state.vscdb` `tokenCount`:** best-effort and **usually `0`** —
  confirmed by Cursor staff (forum thread 155984: the post-stream backend fetch
  "doesn't always work… not the main source of truth… rely on the Usage
  Dashboard"). Do not treat it as authoritative.
- **Real tokens/cost are server-side only** (Cursor Usage Dashboard CSV / Admin
  API) — out of scope here, tracked in **59m1**.

Consequently gevp writes an **empty `tokens_v2`** tree for Cursor sessions.
Cursor pricing is genuinely token-based with published rates (e.g. Composer 2.5
$0.5/$2.5 per-M; Auto $1.25/$6.00, +$0.25/M Cursor Token Rate on non-Auto), so
cost becomes computable *once real tokens land* (59m1) — Cursor is **not** a
flat/subscription-only provider. Do not document or imply local token
availability.

### Model name is not in the JSONL

Model is absent from the JSONL and from `meta.json`. It is recoverable
best-effort from Cursor's app-support `state.vscdb`
(`composerData.modelConfig.modelName`, joined by `composerId == <session-uuid>`)
and passed as sync metadata by the CLI. A second local source,
`~/.cursor/ai-tracking/ai-code-tracking.db`, also carries a `model` column keyed
by `conversationId == <session-uuid>` but **no** token/usage/cost columns — it
can enrich the model name but never the cost.

## Why this differs from Claude Code

| Aspect | Claude Code | Cursor |
|--------|-------------|--------|
| Line envelope | top-level `type`, `uuid`, `timestamp` per line | `{role, message:{content:[…]}}`; no top-level `type`/`uuid`/`timestamp` |
| Parser struct | `TranscriptLine` (`internal/analytics/parser.go`) keys on top-level `type` | new Cursor adapter required (4r41) |
| Per-message timestamp | yes | **no** (session-level `updatedAtMs` only) |
| Tokens / usage | inline `usage` per assistant message | **none** in synced data |
| Model | inline per message | **not** in JSONL (best-effort sync metadata) |
| Tool outputs | `toolUseResult` on user rows | **none** — inputs only |
| Edit tool name | `Edit` / `MultiEdit` | `StrReplace` |
| File-path key | `file_path` | `path` |
| Turn boundary | derived from message sequence | explicit `{type:"turn_ended", status}` rows |

## Test fixtures

`backend/internal/analytics/testdata/cursor/main.jsonl` is a sanitized main-thread
transcript (paths and user text redacted; structural variety preserved). It is
intentionally placed in a new `cursor/` subdirectory under the previously-flat
`analytics/testdata/`, to keep a growing per-provider fixture set tidy.

The fixture is the executable form of this spec. It contains:

- a text-only `user` row;
- `assistant` rows mixing a `text` block with `tool_use` blocks;
- at least one each of `Read`, `Shell`, `Write`, and **`StrReplace`** (the edit
  tool), plus `Grep`, `Glob`, `SemanticSearch`, `Task`, `Delete`, `WebSearch`,
  and `AskQuestion`;
- a `turn_ended` **success** row and a `turn_ended` **error** row.

`cursor_fixture_test.go` (same package) smoke-parses it and asserts these
invariants, including the central "not Claude envelope" guard. The fixture is
readable from the package directory with **no `~/.cursor` access**, satisfying
fy5q's acceptance criterion. Downstream tickets (gevp parser/compute, 18n2
frontend service tests) consume the same fixture.

## Deferred: subagents

Cursor stores subagent transcripts at
`agent-transcripts/<session-uuid>/subagents/<subagent-uuid>.jsonl`. They use the
**identical** line envelope as the main thread, plus a subagent-only
`UpdateCurrentStep` tool (`input` keys: `current_step`, `final_summary`,
`completed_subtitle`). v1 does **not** sync, parse, or render subagents — the
layout is documented here only so the follow-up (subagent upload + analytics +
UI) has the contract. No subagent fixture ships in v1.

## External corroboration

- [Cursor forum — accessing the full agent transcript](https://forum.cursor.com/t/accessing-the-full-agent-transcript-in-cursor/157311):
  agent-transcripts JSONL carries user messages, assistant text, and tool
  **inputs** only — not tool outputs (by design).
- [Cursor forum — CursorDiskKV tokenCount always 0](https://forum.cursor.com/t/cursordiskkv-table-records-always-show-0-for-tokencount/155984):
  Cursor staff confirm local `tokenCount` is best-effort and not the source of
  truth; use the Usage Dashboard for real tokens.
- [tokenuse Cursor docs](https://tokenuse.app/docs/development/tools/cursor/):
  same `{role, message.content[]}` shape; token counts live in **separate**
  SQLite (`state.vscdb`), not in agent-transcripts JSONL.
- Community tools [cursor-trace](https://github.com/dwqs/cursor-trace) and
  [cursor-chat-browser](https://github.com/snehaendait/cursor-chat-browser) read
  the JSONL for content and optionally enrich timestamps from `state.vscdb`.

## Decisions log

| Decision | Resolution |
|----------|------------|
| Fixture location | New `backend/internal/analytics/testdata/cursor/` subdir; main thread only; no separate frontend fixture file. |
| Sync contract v1 | Main `<uuid>.jsonl` → `file_type=transcript` only; subagents deferred. |
| Cost semantics | **TBD** — no tokens/cost in synced data; gevp writes empty `tokens_v2`; real cost tracked in 59m1 once Dashboard/Admin-API tokens land. |
| Subagent aggregation policy | **TBD** — deferred with the rest of subagent support. |
| Edit-tool mapping | `StrReplace` is the Cursor edit tool; map it explicitly in the code-activity card (gevp). |
| Model source | Not in JSONL; best-effort sync metadata from `state.vscdb` (joined by `composerId`). |

## Sibling tickets

- **4r41** — Cursor parser + provider registration (consumes this spec + fixture).
- **gevp** — Cursor analytics compute / cards (consumes this spec + fixture).
- **18n2** — frontend Cursor adapter + service tests (reads the same fixture).
- **59m1** — server-side token/cost ingestion (Dashboard CSV / Admin API).
