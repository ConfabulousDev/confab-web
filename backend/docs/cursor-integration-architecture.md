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
              └── <subagent-uuid>.jsonl   # subagent thread → file_type=agent
```

- `<sanitized-project-path>` is the absolute working-directory path with `/`
  replaced by `-` (e.g. `Users-jackie-dev-confab-web`).
- `<session-uuid>` is a v4 UUID; the directory name and the main file's basename
  are the **same** UUID. This UUID is the canonical Cursor session id.
- The `subagents/` directory uses the identical line envelope as the main thread
  (plus a subagent-only `UpdateCurrentStep` tool). The CLI uploads each subagent
  file as `file_type=agent`; the backend merges its activity into the parent
  session's analytics — see [Subagents](#subagents).

Session metadata lives in a **separate** tree:

```
~/.cursor/chats/<workspace-hash>/<session-uuid>/meta.json
```

See [Session metadata (meta.json)](#session-metadata-metajson).

## Sync contract

| Source file | Maps to |
|-------------|---------|
| `agent-transcripts/<uuid>/<uuid>.jsonl` (main) | `file_type=transcript` |
| `agent-transcripts/<uuid>/subagents/<uuid>.jsonl` | `file_type=agent` |

This mirrors the Codex/OpenCode convention (main file → `transcript`, subagent
files → `agent`). The backend parses subagent files with the same envelope as
the main thread and aggregates their tool/code/agent activity into the parent
session — see [Subagents](#subagents).

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

### Cursor-native `[REDACTED]` in assistant `text`

Cursor appends a **bare** `[REDACTED]` placeholder to nearly every assistant
turn on disk. Across a full local scan it appears in exactly two shapes and
**never** embedded mid-sentence:

- **standalone** — the whole `text` block is `[REDACTED]` (a tool-only turn: the
  assistant emitted just `tool_use` blocks, with `[REDACTED]` standing in for the
  absent narrative); or
- **trailing** — `[REDACTED]` (with surrounding whitespace) appended after the
  real narrative in a `text` block.

> **This is Cursor's own UI scaffolding, NOT Confab CLI secret scrubbing.** It is
> a `tool_use`-adjacent placeholder Cursor writes into the agent-transcripts JSONL
> before upload — distinct from the `tool_use` blocks themselves (which carry the
> real tool name and `input`). It is **not** the Confab CLI `[REDACTED:TYPE]`
> marker (e.g. `[REDACTED:GITHUB_TOKEN]`) that the upload-time secret scrubber
> emits on Claude/Codex transcripts. The bare token has **no** colon/type; a real
> redacted secret always does.

**Confab strips the bare token during normalize** (shipped in fa3h). The strip
helper is `cleanCursorAssistantText` — `frontend/src/services/cursorTranscriptService.ts`
(exported) and mirrored in `backend/internal/analytics/cursor_types.go`. It matches
only a **trailing bare** `[REDACTED]` (regex `\s*\[REDACTED\]\s*$`, no colon/type)
and returns `""` when the block's whole trimmed content was the placeholder, so a
`[REDACTED]`-only assistant turn omits its assistant row while its `tool_use` rows
still render. It is applied everywhere assistant text is surfaced:
`normalizeCursorLines` (frontend display + Cmd-F search), `extractCursorSearchText`
(backend search index), and `joinCursorText` / `PrepareCursorTranscript` (backend
smart recap).

The contrast with the Confab CLI `[REDACTED:TYPE]` marker is deliberate and
load-bearing: the typed markers are a real redacted secret and are **never**
touched by the strip — they stay visible in the transcript and are **counted** by
the `RedactionsAnalyzer` on the providers that emit them. Cursor has no such
analyzer or redactions card, because its bare `[REDACTED]` is noise, not a secret.

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
| `createdAtMs` | Session creation, epoch ms. **Source for `created_at`** (duration start anchor; refines `first_seen`). |
| `updatedAtMs` | Last update, epoch ms. **Source for `latest_message_at`.** Present in every observed file. |
| `hasConversation` | Boolean. |
| `title` | **Optional** — absent in some sessions. The CLI must not assume it exists. |

There are **no** token, usage, cost, or model fields in `meta.json`.

## Known gaps

These are the most-asked questions; the answers are definitive as of Jun 2026.

### No inline timestamps — duration is estimated from session bounds

JSONL lines carry no per-line timestamp. The only time signals are session-level:
`meta.json.createdAtMs` (creation) and `updatedAtMs` (last update). The CLI passes
them as `metadata.created_at` and `metadata.latest_message_at`.

**Estimation contract (5w7r):** exact per-message times are not recoverable from
synced data, but the session **window** is, so timing degrades to an estimate
rather than nothing:

- **Start anchor** = `created_at` ?? `first_seen`. `created_at` is folded into
  `first_seen` at ingest — an earlier `created_at` lowers `first_seen` (never
  raises it); when `meta.json` is absent, `first_seen` (session init time) stands
  in. Far-future values are clamped/dropped (sort-order-abuse guard).
- **End anchor** = `last_message_at` (from `updatedAtMs`) ?? `last_sync_at`.
- **Session duration** (`DurationMs`) = end − start, computed in
  `analytics.ComputeFromCursorRollout` via `CursorSessionBounds`. It stays nil
  when an anchor is missing or the window is non-positive — no invented or
  negative spans.
- **Per-row display timestamps** are interpolated **frontend-side** (ce79) from
  these same bounds over conversation-row index; the stored JSONL is **never**
  rewritten with estimates.

**Accuracy caveats:** the duration is a wall-clock window, not a sum of active
turns — it includes idle gaps and is only as tight as `createdAtMs`/`updatedAtMs`
allow. Per-message estimates are linear (uniform spacing), so they will not match
real keystroke timing. Assistant-vs-user turn timing and utilization are left
**nil** (the window gives no honest basis to split them). Exact bubble-level
timestamps from `state.vscdb` are a separate CLI follow-up.

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

Because the synced JSONL has no model field, the model needs a per-session
read-back path for analytics. The sync handler persists a non-empty
`metadata.model` from a cursor transcript chunk into the **`cursor_session_meta`**
sidecar (PK `session_id`, first-non-empty-model-wins, mirroring the
`codex_rollouts` precedent), and `cursorProvider.Parse` reads it back so
`cards.session.models_used` is `[<model>]` (zsr6). Absent → `[]`, never an
invented model. This populates only the model name; real **tokens/cost** remain
a separate follow-up (59m1) and a `pricing.json` Cursor section becomes
defensible only once real tokens exist.

## Why this differs from Claude Code

| Aspect | Claude Code | Cursor |
|--------|-------------|--------|
| Line envelope | top-level `type`, `uuid`, `timestamp` per line | `{role, message:{content:[…]}}`; no top-level `type`/`uuid`/`timestamp` |
| Parser struct | `TranscriptLine` (`internal/analytics/parser.go`) keys on top-level `type` | new Cursor adapter required (4r41) |
| Per-message timestamp | yes | **no** — estimated from session bounds (`createdAtMs`/`updatedAtMs`); see [No inline timestamps](#no-inline-timestamps--duration-is-estimated-from-session-bounds) |
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

## Subagents

Cursor stores subagent transcripts at
`agent-transcripts/<session-uuid>/subagents/<subagent-uuid>.jsonl`. They use the
**identical** line envelope as the main thread, plus a subagent-only
`UpdateCurrentStep` tool (`input` keys: `current_step`, `final_summary`,
`completed_subtitle`) — a progress marker, **not** a real tool call.

The CLI uploads each subagent file as `file_type=agent` under the parent
session's hosted session (confab ticket 2brd). The backend (`cursor_provider.go`)
lists agent files alongside the main transcript and lazily materializes them on
first compute (`cursorRollout.materialize`), capped at `storage.MaxAgentFiles`;
a per-file download/parse failure is non-fatal and recorded as a
`LineValidationError`. This mirrors the OpenCode subagent path (CF-539).

Analytics aggregation is **asymmetric** (OpenCode parity):

- **Merged across `[main, ...subagents]`:** tools, code activity, agents,
  and the global search index (so a phrase appearing only in a subagent still
  matches the parent session — `cursor_search.go`).
- **Main-thread only:** the conversation card (turn counts), the session
  message counts, the session window / `DurationMs`, and `models_used` —
  subagents nest within the main session window and do not widen it.
- `UpdateCurrentStep` is classified-and-skipped in the tools card: it is neither
  counted as a tool nor surfaced as a tool name.

Transcript rendering stays **main-thread only** (provider parity — Codex and
OpenCode do not render subagent threads in the main pane); subagents contribute
to aggregated analytics cards and search recall, not the transcript view.

The fixture `backend/internal/analytics/testdata/cursor/subagent.jsonl` is the
executable form of this contract: a sanitized subagent transcript with the
main-thread envelope plus two `UpdateCurrentStep` markers,
exercised by `cursor_fixture_test.go` and the compute/precompute tests.

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
| Sync contract | Main `<uuid>.jsonl` → `file_type=transcript`; `subagents/<uuid>.jsonl` → `file_type=agent` (wc9t). |
| Cost semantics | **TBD** — no tokens/cost in synced data; gevp writes empty `tokens_v2`; real cost tracked in 59m1 once Dashboard/Admin-API tokens land. |
| Subagent aggregation policy | **Resolved (wc9t)** — merge tools/code/agents/search across `[main, ...subagents]`; conversation, session counts, bounds, and `models_used` stay main-only; `UpdateCurrentStep` ignored; transcript main-only. Mirrors OpenCode CF-539. |
| Edit-tool mapping | `StrReplace` is the Cursor edit tool; map it explicitly in the code-activity card (gevp). |
| Model source | Not in JSONL; best-effort sync metadata from `state.vscdb` (joined by `composerId`). |

## Sibling tickets

- **4r41** — Cursor parser + provider registration (consumes this spec + fixture).
- **gevp** — Cursor analytics compute / cards (consumes this spec + fixture).
- **18n2** — frontend Cursor adapter + service tests (reads the same fixture).
- **59m1** — server-side token/cost ingestion (Dashboard CSV / Admin API).
