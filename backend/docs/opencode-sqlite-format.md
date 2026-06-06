# OpenCode SQLite Session Storage

## Location

```
~/.local/share/opencode/opencode.db
```

XDG-compliant: `$XDG_DATA_HOME/opencode/opencode.db`. SQLite database using WAL mode with Drizzle ORM.

---

## Schema

### `session` — one row per conversation

| Column | Type | Description |
|---|---|---|
| `id` | TEXT PK | `ses_<ulid>` |
| `project_id` | TEXT FK | Links to `project.id` (project hash) |
| `parent_id` | TEXT? | Forked session parent |
| `slug` | TEXT | URL-friendly name |
| `directory` | TEXT | Working directory on disk |
| `path` | TEXT? | Path within project |
| `title` | TEXT | Auto-generated title |
| `version` | TEXT | OpenCode version |
| `agent` | TEXT? | Agent name ("build", "plan", "explore") |
| `model` | TEXT? | JSON: `{id, providerID, variant?}` |
| `cost` | REAL | Total cost in USD |
| `tokens_input` | INTEGER | Total input tokens |
| `tokens_output` | INTEGER | Total output tokens |
| `tokens_reasoning` | INTEGER | Total reasoning tokens |
| `tokens_cache_read` | INTEGER | Total cache read tokens |
| `tokens_cache_write` | INTEGER | Total cache write tokens |
| `share_url` | TEXT? | Shared session URL |
| `summary_additions` | INTEGER? | Net additions across session |
| `summary_deletions` | INTEGER? | Net deletions across session |
| `summary_files` | INTEGER? | Files changed |
| `summary_diffs` | TEXT? | JSON: array of `FileDiff` |
| `revert` | TEXT? | JSON: `{messageID, partID?, snapshot?, diff?}` |
| `permission` | TEXT? | JSON: permission ruleset |
| `metadata` | TEXT? | JSON: extra metadata |
| `workspace_id` | TEXT? | Workspace reference |
| `time_created` | INTEGER | Unix ms |
| `time_updated` | INTEGER | Unix ms |
| `time_compacting` | INTEGER? | Unix ms of last compaction |
| `time_archived` | INTEGER? | Archive timestamp |

### `message` — one row per conversation turn

| Column | Type | Description |
|---|---|---|
| `id` | TEXT PK | `msg_<ulid>` |
| `session_id` | TEXT FK | → `session.id` CASCADE |
| `time_created` | INTEGER | Unix ms |
| `time_updated` | INTEGER | Unix ms |
| `data` | TEXT (JSON) | **Message payload (see below)** |

Index: `(session_id, time_created, id)`

**`data` JSON fields:**

#### User message (`role: "user"`)
```json
{
  "role": "user",
  "time": { "created": <ms> },
  "agent": "build",
  "model": {
    "providerID": "opencode",
    "modelID": "deepseek-v4-flash-free",
    "variant": "default"
  },
  "system": "<optional system prompt>",
  "tools": { "<toolName>": true, ... },
  "format": { "type": "text" | "json_schema", ... },
  "summary": { "title": "...", "body": "...", "diffs": [...] }
}
```

#### Assistant message (`role: "assistant"`)
```json
{
  "role": "assistant",
  "parentID": "msg_<ulid>",
  "providerID": "opencode",
  "modelID": "deepseek-v4-flash-free",
  "agent": "build",
  "mode": "build",
  "variant": "high",
  "path": { "cwd": "<cwd>", "root": "<project_root>" },
  "finish": "stop" | "tool-calls" | "max_tokens" | "length" | null,
  "cost": 0.0,
  "tokens": {
    "total": <int>,
    "input": <int>,
    "output": <int>,
    "reasoning": <int>,
    "cache": { "read": <int>, "write": <int> }
  },
  "error": { "name": "APIError" | "AbortedError" | ..., "message": "..." },
  "structured": <any>,
  "summary": true | false
}
```

**Key fields for completeness detection:**
- `finish` — when non-null, message is settled. Values: `"stop"`, `"tool-calls"`, `"max_tokens"`, `"length"`
- `error` — when present, message terminated abnormally
- `cost` + `tokens` — final accounting (also in `step-finish` part)

---

### `part` — atomic content units within a message

| Column | Type | Description |
|---|---|---|
| `id` | TEXT PK | `prt_<ulid>` |
| `message_id` | TEXT FK | → `message.id` CASCADE |
| `session_id` | TEXT | Denormalized for query performance |
| `time_created` | INTEGER | Unix ms |
| `time_updated` | INTEGER | Unix ms |
| `data` | TEXT (JSON) | **Part payload (see below)** |

Indexes: `(message_id, id)`, `(session_id)`

---

## Part Types (12 total)

Every part has a `type` discriminator and the common fields `id`, `sessionID`, `messageID`.

### `text` — message text
```json
{
  "type": "text",
  "text": "Hello world",
  "synthetic": true | false,      // auto-generated (e.g. compaction prompt)
  "ignored": true | false,        // excluded from model context
  "time": { "start": <ms>, "end": <ms> },
  "metadata": { ... }
}
```

### `tool` — tool invocation (most complex)
Four state transitions, each an **in-place UPDATE** of the same row:

**pending** — AI requested a tool call
```json
{
  "type": "tool",
  "callID": "call_<n>_ET_<random>",
  "tool": "Bash" | "Read" | "Write" | "Edit" | "Grep" | "Glob" | "Task" | ...,
  "state": {
    "status": "pending",
    "input": { ...tool-specific args... },
    "raw": "<raw JSON of input>"
  }
}
```

**running** — tool is executing
```json
{
  "type": "tool",
  "callID": "...",
  "tool": "Bash",
  "state": {
    "status": "running",
    "input": { "command": "ls" },
    "title": "Running command...",
    "time": { "start": <ms> }
  }
}
```

**completed** — tool finished with output
```json
{
  "type": "tool",
  "callID": "...",
  "tool": "Bash",
  "state": {
    "status": "completed",
    "input": { "command": "ls" },
    "output": "file1\nfile2\n...",       // full output text
    "title": "List files",
    "metadata": { ... },
    "time": { "start": <ms>, "end": <ms>, "compacted": <ms> },
    "attachments": [ { "type": "file", ... } ]
  }
}
```

**error** — tool failed
```json
{
  "type": "tool",
  "callID": "...",
  "tool": "Bash",
  "state": {
    "status": "error",
    "input": { "command": "ls" },
    "error": "command not found",
    "time": { "start": <ms>, "end": <ms> }
  }
}
```

### `reasoning` — model's thinking/chain-of-thought
```json
{
  "type": "reasoning",
  "text": "Let me think about this step by step...",
  "metadata": { "anthropic": { "signature": "..." } },
  "time": { "start": <ms>, "end": <ms> }
}
```

### `file` — file attachment
```json
{
  "type": "file",
  "mime": "image/png" | "text/plain" | "application/pdf" | ...,
  "filename": "screenshot.png",
  "url": "data:image/png;base64,...",
  "source": {
    "type": "file" | "symbol" | "resource",
    "text": { "value": "...", "start": <n>, "end": <n> },
    "path": "/path/to/file",
    "range": { "start": { "line": 1, "character": 0 }, "end": {...} },
    "name": "symbolName",
    "kind": <int>
  }
}
```

### `step-start` — multi-step turn boundary
```json
{
  "type": "step-start",
  "snapshot": "<git commit hash>"
}
```

### `step-finish` — turn boundary with accounting
```json
{
  "type": "step-finish",
  "reason": "tool-calls" | "stop" | "max_tokens" | ...,
  "snapshot": "<git commit hash>",
  "cost": 0.015,
  "tokens": {
    "total": 15000,
    "input": 10000,
    "output": 5000,
    "reasoning": 2000,
    "cache": { "read": 3000, "write": 2000 }
  }
}
```

### `snapshot` — git checkpoint
```json
{
  "type": "snapshot",
  "snapshot": "<git commit hash>"
}
```

### `patch` — file modification
```json
{
  "type": "patch",
  "hash": "<git hash of patch>",
  "files": ["/path/to/file.go"]
}
```

### `agent` — agent switch
```json
{
  "type": "agent",
  "name": "plan" | "build" | "explore",
  "source": { "value": "...", "start": <n>, "end": <n> }
}
```

### `subtask` — subagent task definition
```json
{
  "type": "subtask",
  "prompt": "Search for X",
  "description": "Explore the codebase for X patterns",
  "agent": "explore",
  "model": { "providerID": "opencode", "modelID": "..." },
  "command": "opencode run ..."
}
```

### `compaction` — context window compression
```json
{
  "type": "compaction",
  "auto": true,
  "overflow": true | false,
  "tail_start_id": "msg_<ulid>"   // first message retained after truncation
}
```

### `retry` — error recovery attempt
```json
{
  "type": "retry",
  "attempt": 2,
  "error": { "name": "APIError", "message": "..." },
  "time": { "created": <ms> }
}
```

---

## Ordering Semantics

### Messages
Ordered by **(id ASC)** — monotonically increasing ULIDs (`msg_` prefix). String comparison on `id` gives reliable chronological order. The `time_created` column is redundant for ordering.

### Parts within a message
Ordered by **(id ASC)** — ULIDs (`prt_` prefix). Created in the order they appear in the conversation.

---

## Completeness Detection

There is **no explicit completion flag** in the DB. Instead, use these heuristics:

### 1. Primary: `finish` field (most reliable)
An assistant message is complete when `data->finish` is non-null:
- `"stop"` — model finished naturally
- `"tool-calls"` — model requested tools (results will follow in next user message)
- `"max_tokens"` — hit token limit
- `"length"` — hit length limit

### 2. Secondary: `step-finish` part
A message with a `step-finish` part is complete. This part carries the final `cost` and `tokens` accounting.

### 3. Tool state terminality
A tool part is settled when `state.status` is `"completed"` or `"error"`. Until then, the row exists but is still being updated in place.

### 4. The "next message" heuristic
Once a message with a higher `id` exists, all parts of the prior message are settled. This is the simplest and most robust approach for polling.

### Completeness matrix for a message:
| Signal | User message | Assistant message |
|---|---|---|
| `finish` is set | N/A | ✓ Complete |
| `error` is set | N/A | ✓ Complete (abnormal) |
| `step-finish` part exists | N/A | ✓ Complete |
| Next message exists | ✓ Complete | ✓ Complete |
| All tool parts terminal | N/A | ✓ Tools settled |
| No `step-finish` + no `finish` | — | ✗ Incomplete |

---

## Lifecycle of a Turn

Using a real example from the database, a single assistant turn produces parts in this order:

```
step-start     → boundary, optional git snapshot
reasoning      → model's internal thinking (streamed via events)
text           → response text
tool (bash)    → callID: call_0, state: pending  → running  → completed
tool (grep)    → callID: call_1, state: pending  → running  → completed
text           → more response
step-finish    → reason: "tool-calls", final cost + tokens
```

Each tool part row is **updated in place** through its state transitions. The DB is written:
1. Once when the part is first created (`pending`)
2. Once when execution starts (`running`)
3. Once when execution finishes (`completed` or `error`)

Streaming bytes (tool stdout, reasoning text) flow through **in-memory PartDelta events** (SSE bus), NOT through repeated SQLite writes. The DB only stores the final state.

---

## Comparison: OpenCode Parts vs Other Formats

| Granularity | Claude Code JSONL | Codex JSONL | OpenCode SQLite |
|---|---|---|---|
| **Line / row** | One complete turn (user or assistant) | One event (`response_item`, `event_msg`, etc.) | One atomic content unit (`text`, `tool`, `reasoning`, etc.) |
| **Turn assembly** | Single JSON object with `message.content[]` | `task_started`..`task_complete` lifecycle | Group parts by `message_id`, sort by `part.id ASC` |
| **Tool streaming** | Single `tool_result` block when done | Event-based | In-place row updates: `pending→running→completed` |
| **Completeness** | Line is written when turn ends | `task_complete` event | `finish` field + `step-finish` part |

---

## Note on Legacy File Storage

Before the SQLite migration (pre-v1.14), OpenCode stored data as individual JSON files:

```
~/.local/share/opencode/storage/
├── session/{projectHash}/{sessionID}.json
├── message/{sessionID}/msg_{messageID}.json
├── part/{messageID}/{partID}.json
```

The SQLite schema preserves this entity structure — the old JSON file content maps directly to the `data` column in each table row.
