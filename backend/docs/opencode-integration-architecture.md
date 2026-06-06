# OpenCode Integration Architecture

## Overview

Add OpenCode as a third provider (alongside Claude Code and Codex). OpenCode has no hook system, so we use a **hybrid approach**: a minimal TypeScript plugin for lifecycle events (start/stop) + the existing Go daemon pattern for data sync.

## Architecture

```
OpenCode process
 ├── TypeScript Plugin (confab-sync.ts)
 │    ├── session.created → spawn Go daemon with serverUrl
 │    ├── session.idle    → no-op (daemon stays alive)
 │    └── dispose         → SIGTERM daemon
 │
 └── HTTP Server (localhost:<port>)
      ├── GET /event          → SSE event bus
      ├── GET /session        → list sessions
      ├── GET /session/:id    → session detail
      └── GET /session/:id/message → messages + parts

Confab Go daemon (background process)
 ├── subscribe SSE at {serverUrl}/event
 ├── poll GET /session, GET /session/:id/message
 ├── upload chunks to Confab backend POST /sync/chunk
 ├── health check on disconnect → self-destruct
 └── parent PID death → self-destruct
```

## Plugin: Minimal Lifecycle Bridge

**Location:** `~/.config/opencode/plugins/confab-sync.ts`

The plugin is tiny (~20-30 lines). Its only job is to bridge the lifecycle gap since OpenCode has no native hooks.

### Plugin Context

```typescript
type PluginInput = {
  serverUrl: URL      // e.g. http://localhost:4096 (includes port)
  client: SDKClient   // pre-configured HTTP client
  $: BunShell         // for spawning the daemon
  directory: string
  worktree: string
}
```

The `serverUrl` field gives us the port for free — no port discovery needed. The user can run `opencode --port 5151` and the plugin will pass the correct URL to the daemon.

### Events & Actions

| Event | Action |
|-------|--------|
| `session.created` | Idempotent spawn: `confab hook session-start --provider opencode --server-url {url}` |
| `dispose` (cleanup) | Kill daemon via SIGTERM |

The daemon monitors the OpenCode server independently — it doesn't rely on the plugin for per-session events. The plugin is purely for daemon lifecycle (start when first session appears, stop when OpenCode exits).

## Daemon: Data Sync Engine

**Location:** `../confab/pkg/provider/opencode.go` (+ supporting files)

The daemon follows the same pattern as Claude Code/Codex daemons but connects to OpenCode's HTTP API instead of reading JSONL files.

### Startup

```
confab hook session-start --provider opencode --server-url http://localhost:4096
```

1. Parse `--server-url` flag to get `hostname:port`
2. Verify server is reachable: `GET {serverUrl}/global/health`
3. Save state file with `server_url`, `parent_pid`
4. Enter main loop

### Main Loop

1. **SSE Subscription:** `GET {serverUrl}/event`
   - Persistent connection, auto-reconnect with backoff
   - Event types we care about:
     - `server.connected` — initial connection confirmation
     - `session.created` — new session to track (catch up via list)
     - `session.updated` — session metadata changed
     - `session.idle` — session completed, do final sync
     - `session.deleted` — remove from tracking
     - `message.part.updated` — new/changed content (optional optimization)
   - On reconnect, re-list all sessions to catch missed events

2. **Session Discovery:** `GET /session`
   - Compare with tracked sessions in state file
   - Track new sessions, sync them
   - Filter out already-synced sessions

3. **Message Fetching:** `GET /session/{id}/message`
   - Returns messages with their parts
   - Each message has token counts, cost, timestamps
   - Parts include text, tool calls, reasoning, files, etc.

4. **Chunk Upload:** Assemble messages into chunks, POST to Confab

### Shutdown

Three ways the daemon stops:

| Trigger | How |
|---------|-----|
| Parent PID death | OpenCode process exits → signal 0 check fails → shutdown |
| Plugin dispose | Plugin calls `dispose` → sends SIGTERM to daemon |
| Server unreachable | Health check fails after N retries → shutdown |

On shutdown:
1. Final sync of any pending sessions
2. Send `session_end` event for active sessions
3. Clean up state file

## Data Format: Mapping OpenCode to Chunks

### OpenCode API Response Shape

```typescript
// Message (from GET /session/:id/message)
type Message = UserMessage | AssistantMessage

type UserMessage = {
  id: string; sessionID: string; role: "user";
  time: { created: number };
  summary?: { title?: string; body?: string; diffs: FileDiff[] };
  agent: string;
  model: { providerID: string; modelID: string };
}

type AssistantMessage = {
  id: string; sessionID: string; role: "assistant";
  time: { created: number; completed?: number };
  providerID: string; modelID: string;
  cost: number;
  tokens: { input: number; output: number; reasoning: number;
            cache: { read: number; write: number } };
  finish?: string;
}

// Part (included in message detail)
type Part = TextPart | ToolPart | ReasoningPart | FilePart
          | StepStartPart | StepFinishPart | SnapshotPart
          | PatchPart | AgentPart | RetryPart | CompactionPart
          | SubtaskPart
```

### Chunk Serialization Strategy

OpenCode's parts are more granular than Claude Code's per-turn JSONL lines. Two options:

#### Option A: Assemble into message-level JSONL (recommended)

One chunk line per conversation message (user or assistant). Parts are embedded within the message:

```json
{
  "type": "assistant",
  "timestamp": "2026-06-01T10:30:05Z",
  "message_id": "msg_abc",
  "providerID": "opencode",
  "modelID": "claude-sonnet-4-20250514",
  "parts": [
    { "type": "text", "text": "Here's the fix..." },
    { "type": "tool", "name": "edit", "input": {"filePath": "src/main.go"} }
  ],
  "tokens": { "input": 1500, "output": 423, "cache_read": 500, "cache_write": 0 },
  "cost": 0.0215
}
```

This matches the Claude Code JSONL structure closely — the backend already handles per-line timestamp extraction and PR link detection from similar formats.

#### Option B: One line per part

More granular, but requires the backend/analytics to understand part-level grouping:

```json
{"type": "part", "message_id": "msg_abc", "part_type": "text", "timestamp": "...", "text": "Here's the fix..."}
{"type": "part", "message_id": "msg_abc", "part_type": "tool", "timestamp": "...", "name": "edit", "input": {...}}
```

**Recommendation: Option A.** It's closest to the existing Claude Code parser and avoids major backend changes to the analytics pipeline. The daemon assembles parts into messages before uploading.

### Conversation Turn Assembly

OpenCode stores messages and parts separately. The daemon must:

1. Fetch `GET /session/{id}/message` for each session
2. Group parts by `message_id`
3. Assemble each message with its parts inline
4. Add a `"timestamp"` field (required by backend) from `message.time.created`
5. Serialize as a JSONL line

## File-by-File Plan

### Confab CLI (`../confab`)

| File | Purpose |
|------|---------|
| `pkg/provider/provider.go` | Add `NameOpencode = "opencode"` constant, register in `registry` |
| `pkg/provider/opencode.go` | `Opencode` provider struct implementing `Provider` interface |
| `pkg/provider/opencode_discovery.go` | `ScanSessions` via `GET /session`, `FindSessionByID` |
| `pkg/provider/opencode_session.go` | Session parsing, message assembly, chunk serialization |
| `pkg/provider/detect.go` | Add `checkOpenCode()` to `DetectInstalled()` |
| `cmd/hook_sessionstart.go` | Handle `--server-url` flag for OpenCode |
| `pkg/daemon/daemon.go` | Maybe minor tweaks for SSE-based event loop |
| `pkg/sync/engine.go` | Maybe minor tweaks for OpenCode metadata |

No changes needed to `hookconfig/` (no settings.json/config.toml to modify).

### Confab Backend (`confab-web/backend`)

| File | Purpose |
|------|---------|
| `internal/models/provider.go` | Add `ProviderOpencode`, `"opencode"` to `CanonicalProviders` |
| `internal/analytics/provider.go` | Register `opencodeProvider` |
| `internal/analytics/opencode_provider.go` | Provider adapter: `Parse()`, `ComputeCards()`, `DisplayName()` |
| `internal/analytics/pricing.go` | OpenCode pricing (delegates to LLM provider prices) |

OpenCode supports 75+ LLM providers, so the "provider" in Confab's sense is "opencode" regardless of which model the user chose. The model-specific pricing lives in the message-level `modelID` field.

### Confab Frontend (`confab-web/frontend`)

| File | Purpose |
|------|---------|
| `src/utils/providers.ts` | Add OpenCode metadata (icon, label, brand color) |
| `src/providers/opencode.ts` | OpenCode adapter implementing `ProviderAdapter` |
| `src/utils/tokenStats.ts` | OpenCode model prices (keyed by modelID) |

## Comparison with Existing Providers

| Aspect | Claude Code | Codex | OpenCode |
|--------|-------------|-------|----------|
| **Data source** | JSONL file | JSONL file | HTTP API + SSE |
| **Daemon trigger** | SessionStart hook | SessionStart hook | Plugin `session.created` |
| **Daemon stop** | SessionEnd hook + SIGTERM | Parent PID death | Plugin `dispose` + health check |
| **Session ID** | Short alphanumeric | UUID | Short alphanumeric |
| **Tree walk** | N/A (single session) | SQLite thread_spawn_edges via WalkUpToRoot | N/A (single session, subagents via parentID?) |
| **InitTranscript** | No-op | Read session_meta JSON | N/A (API returns all data) |
| **DiscoverDescendants** | No-op | SQLite subagent tree | N/A (API lists all sessions) |
| **AnnotateChunk** | Summary + first message | CodexRolloutMetadata | Tokens + cost + model |
| **Port/host discovery** | N/A (file-based) | N/A (file-based) | Plugin passes `serverUrl` |
| **Agent discovery** | Parse transcript JSONL | Query SQLite | Message `agent` field |

## Open Questions

1. **Subagents:** OpenCode has `agent` and `subtask` part types. Does the daemon track subagent sessions as separate `sync_files` (like Codex) or inline them in the parent message parts?
2. **SSE event replay:** On SSE reconnect, how far back does OpenCode buffer events? Do we need to re-list all sessions?
3. ~~**Token cost:** OpenCode API returns `cost` per message — do we use this value directly or compute from model pricing tables?~~ **Resolved (hybrid):** prefer OpenCode's reported per-message `info.cost`, summed per `(providerID, modelID)`; fall back to Confab's pricing table only when a group reports `0`. This is correct across OpenCode's 75+ providers (most unpriced by our table, which would otherwise bill `$0`) while staying robust when cost is unreported. See `computeOpenCodeTokens` in `internal/analytics/opencode_compute.go`.
4. **File diffs:** OpenCode tracks `FileDiff` at the session level (`session.diff` SSE event) — should these be extracted for the summary card?
