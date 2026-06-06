# OpenCode Phase 2 — Web Implementation Plan

## Scope

This plan covers the **confab-web** repo (backend + frontend). The CLI Phase 2 work (daemon HTTP sync, `ScanSessions`, `FindSessionByID`) is planned separately in the sibling `confab` repo.

**Assumption:** The CLI daemon delivers OpenCode session data to the backend via the existing sync protocol, serialized as one JSONL line per message.

---

## Implementation Status (as built — 2026-06-06)

This plan was written ahead of implementation; the sections below are the
original design. What actually shipped on the `opencode-phase2-backend-wip`
branch differs in a few deliberate ways — read this first:

- **Backend: complete and wired end-to-end.** `opencodeProvider` implements
  `Parse` (loads the `transcript` sync file, merges S3 chunks, deserializes one
  `OpenCodeMessage` per JSONL line), `ComputeCards`, `SearchText`, and
  `PrepareTranscript` (smart-recap XML with `idMap` → stable message ULIDs).
- **Analyzers are consolidated, not split.** The "8 per-card analyzer files"
  in §4 / the File Manifest were implemented as a single
  `internal/analytics/opencode_compute.go` (orchestrated by
  `ComputeFromOpenCodeRollout`). There are no `analyzer_*_opencode.go` files;
  the per-card test files are named after the cards they cover.
- **Token cost is hybrid, not pure recompute.** Prefer OpenCode's reported
  per-message `info.cost` (summed per provider+model); fall back to the pricing
  table only when a group reports `0`. Correct across OpenCode's 75+ providers;
  resolves open question #3 in `opencode-integration-architecture.md`.
- **`tokens_v2` is a universal peer card (always written).** It is written for
  every session — with empty `by_provider` for providers that don't yet build
  the per-model tree (Claude/Codex) — so it participates in `Cards.AllValid` and
  `FindStaleSessions` exactly like the other cards, mirroring the Workflows
  card's "always written, empty for N/A sessions" pattern. The API serves it
  only when it has provider data, so non-OpenCode responses are unchanged and
  the frontend keeps showing the flat `tokens` card for them. The flat `tokens`
  card is still computed for OpenCode (it feeds trends/org aggregation); the
  session view suppresses it when `tokens_v2` has data. Long-term, `tokens_v2`
  will replace the flat `tokens` card for all providers. See
  `internal/analytics/README.md` → "Always-written cards".
- **Messages are parsed once.** `OpenCodeMessage.Parts` is `[]OpenCodePart` and
  `OpenCodePart.State` is `*OpenCodeToolState` (parsed in `Parse`), not
  re-unmarshaled per analyzer.
- **Frontend transcript rendering is NOT built yet.** The §3 adapter,
  §5 `OpenCodeFilterDropdown`, and §6 `OpenCodeTranscriptPane` are **stubs**
  (`opencodeAdapter` returns empty data; `TranscriptPane`/`FilterDropdown`
  render `null`). An OpenCode session currently renders its summary cards
  (including `tokens_v2`) but a blank transcript pane. Full transcript
  rendering + deep-link landing is deferred to a later phase. What *is* wired
  on the frontend: provider registration, the `TokensV2Card` + Zod schema +
  registry entry, the OpenCode icon, and pricing.
- **User-facing docs / marketing copy intentionally NOT updated.** The §Docs
  changes (provider page, FAQ, "OpenCode supported" copy) are held until the
  CLI daemon ships and data flows end-to-end. The provider IS already visible
  in the UI provider filters and the home-page "Works with" list, however,
  because `opencode` was added to `PROVIDER_VALUES`.

---

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Data source | HTTP API + SSE (CLI-side) | OpenCode has no JSONL files; SSE provides real-time events |
| Sync timing | Incremental (real-time) | Matches Claude Code/Codex behavior; users see analytics update live |
| JSONL format | One line per message: `{ info: Message, parts: Part[] }` | Mirrors OpenCode's HTTP API response shape exactly |
| Pricing | Multi-provider (major providers first: Anthropic, OpenAI, Google, DeepSeek, xAI, Mistral) | Complete coverage for common providers; expand based on usage |
| Token semantics | Provider-aware (detect from `providerID`/`modelID`) | Correct cost calculation across Anthropic vs OpenAI vs others |
| Tokens card | New `tokens_v2` card (parallel to existing `tokens`), OpenCode-only for now | Hierarchical display without breaking existing card; Claude/Codex follow-up |
| tokens_v2 schema | Nested provider → model tree | Backend does grouping; frontend renders the tree |
| tokens_v2 storage | New DB table `session_card_tokens_v2` with JSONB column | Nested tree doesn't fit flat columns |
| Subtasks | Count as agents (no outcome signal) | `subtask` parts lack success/error status |
| transcript_path | Synthetic path from daemon (no backend change) | Daemon sends `opencode/<session-id>/messages.jsonl`; backend treats it like any other path |
| Transcript categories | Minimal MVP: User, Assistant, Tool (3 categories) | Iterate from minimal base |
| Transcript API | Raw JSONL pass-through | Matches existing pattern; frontend adapter parses |
| Smart recap | Include in Phase 2 | Transcript XML preparation is straightforward |
| Implementation | Single PR | Complete feature, shipped together |
| OpenCode icon | Official OpenCode branding | Use their logo and brand color |

---

## Data Flow

```
OpenCode (local)
  │
  ├── SQLite DB (~/.local/share/opencode/opencode.db)
  │
  └── HTTP API + SSE (localhost:4096)
        │
        ├── GET /global/event (SSE stream)
        │     └── message.updated, message.part.updated, session.*
        │
        └── GET /session/{id}/message/{messageID}
              └── Returns { info: Message, parts: Part[] }
                    │
                    ▼
              Confab Daemon (CLI)
                    │
                    ├── Assembles complete messages from SSE
                    ├── Fetches authoritative state via REST
                    ├── Serializes as one JSONL line per message
                    │
                    └── POST /api/v1/sync/chunk
                          └── lines: [json_line_1, json_line_2, ...]
                                │
                                ▼
                          Confab Backend
                                │
                                ├── Stores raw JSONL in S3
                                │
                                └── Precomputer (async)
                                      ├── Parse() → opencodeRollout
                                      ├── ComputeCards() → 8 cards
                                      ├── PrepareTranscript() → smart recap
                                      └── SearchText() → search index
```

---

## JSONL Line Format

Each line is a complete message matching OpenCode's HTTP API response shape:

```json
{
  "info": {
    "id": "msg_01JX...",
    "sessionID": "ses_01JX...",
    "role": "assistant",
    "parentID": "msg_01JW...",
    "modelID": "claude-sonnet-4-20250514",
    "providerID": "anthropic",
    "mode": "build",
    "path": { "cwd": "/home/user/project", "root": "/home/user/project" },
    "finish": "tool-calls",
    "cost": 0.015,
    "tokens": {
      "input": 10000,
      "output": 5000,
      "reasoning": 2000,
      "cache": { "read": 3000, "write": 2000 }
    },
    "time": { "created": 1717689600000, "completed": 1717689605000 }
  },
  "parts": [
    {
      "id": "prt_...",
      "sessionID": "ses_...",
      "messageID": "msg_...",
      "type": "step-start",
      "snapshot": "abc123"
    },
    {
      "id": "prt_...",
      "type": "reasoning",
      "text": "Let me check the files...",
      "time": { "start": 1717689600100, "end": 1717689601000 }
    },
    {
      "id": "prt_...",
      "type": "tool",
      "callID": "call_0_ET_abc",
      "tool": "Bash",
      "state": {
        "status": "completed",
        "input": { "command": "ls" },
        "output": "file1\nfile2",
        "title": "List files",
        "time": { "start": 1717689601100, "end": 1717689601500 }
      }
    },
    {
      "id": "prt_...",
      "type": "text",
      "text": "I found 2 files."
    },
    {
      "id": "prt_...",
      "type": "step-finish",
      "reason": "tool-calls",
      "cost": 0.015,
      "tokens": {
        "input": 10000,
        "output": 5000,
        "reasoning": 2000,
        "cache": { "read": 3000, "write": 2000 }
      }
    }
  ]
}
```

### User message shape

```json
{
  "info": {
    "id": "msg_01JW...",
    "sessionID": "ses_01JX...",
    "role": "user",
    "agent": "build",
    "model": { "providerID": "anthropic", "modelID": "claude-sonnet-4-20250514" },
    "time": { "created": 1717689500000 }
  },
  "parts": [
    {
      "id": "prt_...",
      "type": "text",
      "text": "Find all Go files in the project"
    }
  ]
}
```

### Completeness detection

A message line is emitted only when:
- **Assistant**: `info.finish` is non-null (`"stop"`, `"tool-calls"`, `"max_tokens"`, `"length"`) or `info.error` is present
- **User**: Always complete on arrival
- **Tool parts**: Only terminal states (`"completed"` or `"error"`) are included

---

## Backend Changes

### 1. Model Constant

**File:** `backend/internal/models/provider.go`

```go
ProviderOpencode = "opencode"
```

Add to `CanonicalProviders` and `AllowedProviders`.

### 2. Sync Protocol: Synthetic transcript_path

**No backend changes needed.** The daemon sends a synthetic `transcript_path` like `opencode/<session-id>/messages.jsonl`. The backend treats it identically to Claude/Codex paths — S3 key derivation, chunk continuity, precomputer download all work unchanged.

### 3. OpenCode Provider (`SessionProvider` implementation)

**New file:** `backend/internal/analytics/opencode_provider.go`

```go
type opencodeProvider struct{}

func init() {
    RegisterProvider(ProviderOpencode, opencodeProvider{})
}

type opencodeRollout struct {
    Messages []*OpenCodeMessage
}

type OpenCodeMessage struct {
    Info  OpenCodeMessageInfo   `json:"info"`
    Parts []OpenCodePart        `json:"parts"`
    Raw   json.RawMessage       // for redaction scanning
}

type OpenCodeMessageInfo struct {
    ID         string            `json:"id"`
    SessionID  string            `json:"sessionID"`
    Role       string            `json:"role"`
    ParentID   string            `json:"parentID,omitempty"`
    ModelID    string            `json:"modelID,omitempty"`
    ProviderID string            `json:"providerID,omitempty"`
    Mode       string            `json:"mode,omitempty"`
    Agent      string            `json:"agent,omitempty"`
    Finish     *string           `json:"finish,omitempty"`
    Cost       float64           `json:"cost"`
    Tokens     OpenCodeTokens    `json:"tokens"`
    Error      *OpenCodeError    `json:"error,omitempty"`
    Time       OpenCodeTime      `json:"time"`
}

type OpenCodeTokens struct {
    Input     int64             `json:"input"`
    Output    int64             `json:"output"`
    Reasoning int64             `json:"reasoning"`
    Cache     OpenCodeCache     `json:"cache"`
}

type OpenCodeCache struct {
    Read  int64 `json:"read"`
    Write int64 `json:"write"`
}

type OpenCodePart struct {
    ID        string          `json:"id"`
    Type      string          `json:"type"`
    SessionID string          `json:"sessionID,omitempty"`
    MessageID string          `json:"messageID,omitempty"`
    Data      json.RawMessage `json:"-"` // full part JSON for type-specific parsing
}
```

`Parse()` downloads S3 chunks, merges, deserializes each line into `OpenCodeMessage`, returns `*opencodeRollout`.

### 4. Per-Card Analyzers (8 files)

#### `analyzer_tokens_opencode.go`

- Iterate assistant messages with `finish != nil`
- Accumulate `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CacheCreationTokens` from `info.tokens`
- **Provider-aware pricing**: use `info.providerID` + `info.modelID` to look up pricing via `GetPricing()`
- Apply correct token semantics per provider (Anthropic: cache_write billed; OpenAI: writes free, cached is subset of input)
- `FastTurns`/`FastCostUSD` always 0 (OpenCode has no fast mode)
- **tokens_v2 data**: build per-provider → per-model breakdown tree (see tokens_v2 section below)

#### `analyzer_session_opencode.go`

- `UserMessages`: count messages where `info.role == "user"`
- `AssistantMessages`: count messages where `info.role == "assistant"`
- `HumanPrompts` = `UserMessages` (no tool-result user messages in OpenCode)
- `ToolResults`: count `tool` parts with terminal state
- `TextResponses`: count assistant messages with at least one `text` part
- `ThinkingBlocks`: count assistant messages with at least one `reasoning` part
- `ToolCalls`: count `tool` parts with `state.status == "completed"` or `"error"`
- `DurationMs`: `max(time_created) - min(time_created)` across all messages
- `ModelsUsed`: collect unique `info.modelID` values
- `CompactionAuto`/`CompactionManual`: count `compaction` parts by `auto` field

#### `analyzer_tools_opencode.go`

- Iterate all `tool` parts across all messages
- Tool name from `part.tool` field (e.g., `"Bash"`, `"Read"`, `"Write"`, `"Edit"`, `"Grep"`, `"Glob"`, `"Task"`)
- Success: `state.status == "completed"`
- Error: `state.status == "error"`
- Skip non-terminal states (`pending`, `running`)

#### `analyzer_code_activity_opencode.go`

- `FilesRead`: count `tool` parts with `tool == "Read"`, extract `state.input.file_path`
- `FilesModified`: count unique file paths from `Write` and `Edit` tool inputs
- `LinesAdded`/`LinesRemoved`: parse `Write` content (count lines), parse `Edit` old_string/new_string diff
- `SearchCount`: count `tool` parts with `tool == "Grep"` or `tool == "Glob"`
- `LanguageBreakdown`: track file extensions from all file paths
- Supplementary: `patch` parts list modified files (use for validation)

#### `analyzer_conversation_opencode.go`

- Walk messages chronologically by `info.time.created`
- User messages open turn windows; assistant messages close them
- Use `info.time.completed` (if present) for precise assistant end time
- `reasoning` parts have `time.start`/`time.end` for extended activity anchors
- Main session only (exclude subtask sessions)

#### `analyzer_agents_and_skills_opencode.go`

- `TotalAgentInvocations`: count `subtask` parts
- Per-agent stats: group by `part.agent` field (e.g., `"explore"`, `"build"`, `"plan"`)
- Success/Error: always unknown (no outcome signal) — report as invoked without status
- `TotalSkillInvocations`: always 0 (OpenCode has no skill concept)
- `agent` parts (mode switches) are NOT counted as invocations

#### `analyzer_redactions_opencode.go`

- Recursive JSON walk of `RawData` (full parsed JSON) for every message line
- Regex `\[REDACTED:([A-Z][A-Z0-9_]*)\]` on all string values
- Same approach as Claude's redaction analyzer

#### `analyzer_smart_recap_opencode.go`

- `PrepareOpenCodeTranscript`: iterate messages in order, emit XML:
  - User messages → `<user id="msg_...">text from text parts</user>`
  - Assistant messages → `<assistant id="msg_...">` wrapping:
    - `<thinking>reasoning text</thinking>`
    - Text content
    - `<tools_called>tool_name</tools_called>`
    - `<tool_result>output (truncated)</tool_result>`
  - Compaction markers → `<compaction />`
- Message IDs are stable ULIDs → `ClearMessageIDs() = false` (deep-link anchors work)
- Use existing smart recap LLM pipeline (provider-agnostic prompt)

### 5. Orchestrator

**New file:** `backend/internal/analytics/opencode_compute.go`

```go
func (opencodeProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
    r := rollout.(*opencodeRollout)
    result := &ComputeResult{}
    computeOpenCodeTokens(result, r)
    computeOpenCodeSession(result, r)
    computeOpenCodeTools(result, r)
    computeOpenCodeCodeActivity(result, r)
    computeOpenCodeConversation(result, r)
    computeOpenCodeAgentsAndSkills(result, r)
    computeOpenCodeRedactions(result, r)
    return result
}
```

### 6. Search Index

**New file:** `backend/internal/analytics/opencode_search.go`

- Extract user message text, assistant text parts, tool names + truncated inputs
- Same weight-C approach as Codex search

### 7. Pricing

**File:** `backend/internal/pricingsource/pricing.json`

Add `"opencode"` provider block containing major LLM provider model families:
- **Anthropic**: Claude 4 Opus, Claude 4 Sonnet, Claude 3.5 Sonnet/Haiku, Claude 3 Opus
- **OpenAI**: GPT-4o, GPT-4o-mini, o1, o1-mini, o3, o3-mini, o4-mini
- **Google**: Gemini 2.5 Pro, Gemini 2.5 Flash, Gemini 2.0 Flash
- **DeepSeek**: DeepSeek V3, DeepSeek R1
- **xAI**: Grok 3, Grok 3 Mini
- **Mistral**: Mistral Large, Mistral Small, Codestral

Expand based on real usage data from OpenCode sessions.

The tokens analyzer uses `info.providerID` to select the right pricing semantics:
- `anthropic`: cache_write is independent (billed), cache_read is discounted
- `openai`: cache_write is free, cached_input is subset of input
- `google`: follow Google's pricing model
- Others: treat as flat input/output pricing

### 8. tokens_v2 Card

#### Database Migration

New table `session_card_tokens_v2`:

```sql
CREATE TABLE session_card_tokens_v2 (
    session_id    UUID PRIMARY KEY REFERENCES sessions(id),
    version       INTEGER NOT NULL DEFAULT 1,
    computed_at   TIMESTAMPTZ NOT NULL,
    up_to_line    BIGINT NOT NULL DEFAULT 0,
    data          JSONB NOT NULL
);
```

#### Backend Schema

```go
type TokensV2CardRecord struct {
    SessionID  string          `json:"session_id"`
    Version    int             `json:"version"`
    ComputedAt time.Time       `json:"computed_at"`
    UpToLine   int64           `json:"up_to_line"`
    Data       TokensV2Data    `json:"data"` // stored as JSONB
}

type TokensV2Data struct {
    TotalCostUSD  string                    `json:"total_cost_usd"`
    TotalInput    int64                     `json:"total_input"`
    TotalOutput   int64                     `json:"total_output"`
    ByProvider    map[string]TokensV2Provider `json:"by_provider"`
}

type TokensV2Provider struct {
    CostUSD string                     `json:"cost_usd"`
    Models  map[string]TokensV2Model   `json:"models"`
}

type TokensV2Model struct {
    Input       int64  `json:"input"`
    Output      int64  `json:"output"`
    CacheRead   int64  `json:"cache_read"`
    CacheWrite  int64  `json:"cache_write"`
    Reasoning   int64  `json:"reasoning"`
    CostUSD     string `json:"cost_usd"`
}
```

#### API Response

```json
{
  "cards": {
    "tokens_v2": {
      "total_cost_usd": "1.23",
      "total_input": 150000,
      "total_output": 50000,
      "by_provider": {
        "anthropic": {
          "cost_usd": "0.95",
          "models": {
            "claude-sonnet-4-20250514": {
              "input": 100000,
              "output": 30000,
              "cache_read": 20000,
              "cache_write": 5000,
              "reasoning": 10000,
              "cost_usd": "0.95"
            }
          }
        },
        "openai": {
          "cost_usd": "0.28",
          "models": {
            "gpt-4o": {
              "input": 50000,
              "output": 20000,
              "cache_read": 10000,
              "cache_write": 0,
              "reasoning": 0,
              "cost_usd": "0.28"
            }
          }
        }
      }
    }
  }
}
```

#### Compute Logic

The tokens analyzer builds the `TokensV2Data` tree during the same pass as the flat `TokensResult`:
- Group assistant messages by `(providerID, modelID)`
- For each group: sum tokens, compute cost using provider-appropriate semantics
- Aggregate into `ByProvider` → `Models` tree
- Sum all groups for `TotalCostUSD`, `TotalInput`, `TotalOutput`

#### Backward Compatibility

- Existing `tokens` card continues to be computed and served for all providers
- `tokens_v2` is computed alongside `tokens` for OpenCode sessions only
- Frontend renders `tokens_v2` when present, falls back to `tokens`
- Claude/Codex tokens_v2 support is a follow-up PR

---

## Frontend Changes

### 1. Provider Registration

**File:** `frontend/src/utils/providers.ts`
- Add `'opencode'` to `PROVIDER_VALUES`
- Add OpenCode entry to `PROVIDER_METADATA` (label, icon, color)

**File:** `frontend/src/utils/tokenStats.ts`
- Add `'opencode': {}` to default `activePricing` initialization

### 2. OpenCode Icon

**File:** `frontend/src/components/icons.tsx`
- Add `OpenCodeIcon` export using official OpenCode branding

### 3. Provider Adapter

**New file:** `frontend/src/providers/opencodeAdapter.tsx`

```typescript
type OpenCodeRawLine = { info: OpenCodeMessageInfo; parts: OpenCodePart[] }
type OpenCodeRenderItem = OpenCodeRawLine  // identity for MVP
type OpenCodeFilterState = { user: boolean; assistant: boolean; tool: boolean }
type OpenCodeToggles = { toggleCategory: (cat: OpenCodeCategory) => void }
type OpenCodeHierarchicalCounts = {
  user: number
  assistant: number
  tool_call: number
}

type OpenCodeAdapter = ProviderAdapter<
  OpenCodeRawLine,
  OpenCodeRenderItem,
  OpenCodeFilterState,
  OpenCodeToggles,
  OpenCodeHierarchicalCounts
>
```

Key adapter methods:
- `fetchInitial`: fetch transcript JSONL, parse each line into `OpenCodeRawLine`
- `fetchIncremental`: fetch new lines, parse
- `normalize`: identity (raw = item for MVP)
- `extractModel`: read `info.modelID` from first assistant message
- `computeMeta`: min/max `info.time.created` across messages
- `calculateMessageCost`: use `info.cost` directly (OpenCode reports cost) or compute from pricing
- `tokensCostTooltip`: "Cost computed from per-model pricing across all providers used"
- `supportsTILs`: `false`

### 4. Transcript Categories (Minimal MVP)

**New file:** `frontend/src/components/session/opencodeCategories.ts`

Three categories:

| Category | Source | Color |
|----------|--------|-------|
| `user` | Messages with `info.role == "user"` | green |
| `assistant` | Messages with `info.role == "assistant"` (text + reasoning parts) | blue |
| `tool` | Tool parts within assistant messages (extracted as separate render items) | amber |

Default filter state: all visible.

### 5. Filter Dropdown

**New file:** `frontend/src/components/session/OpenCodeFilterDropdown.tsx`

Three flat checkboxes: User, Assistant, Tool Call. Same shared styling as Claude/Codex dropdowns.

### 6. Transcript Pane

**New file:** `frontend/src/components/session/OpenCodeTranscriptPane.tsx`

- Virtual scrolling via `@tanstack/react-virtual`
- Per-message render dispatch:
  - User messages: render text parts
  - Assistant messages: render reasoning (collapsible), text, tool calls (with status badges)
- Deep-link by message ID (`msg_...` ULID)
- Cost mode: show per-message cost from `info.cost`
- Timeline bar with turn-based navigation

### 7. Registry Entry

**File:** `frontend/src/providers/registry.ts`
- Add `REGISTRY['opencode'] = opencodeAdapter as unknown as OpaqueAdapter`

### 8. tokens_v2 Card Component

**New file:** `frontend/src/components/session/cards/TokensV2Card.tsx`

Hierarchical display:
1. **Total cost** row (top level)
2. **Total input/output** tokens (top level)
3. **Per-provider sections** (collapsible):
   - Provider name + provider cost
   - **Per-model rows** (expandable):
     - Input, Output, Cache Read, Cache Write, Reasoning tokens
     - Model cost
4. Provider-specific token formatting (e.g., OpenAI shows cached as subset, Anthropic shows cache write as independent)

**New file:** `frontend/src/components/session/cards/TokensV2Card.stories.tsx`

Stories: Default, SingleProvider, MultiProvider, ZeroCost, HighUsage, Loading.

**Card registry:** Add `tokens_v2` with `shouldRender: (data) => data != null`.

### 9. Zod Schema

**File:** `frontend/src/schemas/api.ts`

```typescript
const TokensV2ModelSchema = z.object({
  input: z.number(),
  output: z.number(),
  cache_read: z.number(),
  cache_write: z.number(),
  reasoning: z.number(),
  cost_usd: z.string(),
});

const TokensV2ProviderSchema = z.object({
  cost_usd: z.string(),
  models: z.record(z.string(), TokensV2ModelSchema),
});

const TokensV2CardDataSchema = z.object({
  total_cost_usd: z.string(),
  total_input: z.number(),
  total_output: z.number(),
  by_provider: z.record(z.string(), TokensV2ProviderSchema),
});

// Add to AnalyticsCardsSchema:
tokens_v2: TokensV2CardDataSchema.optional(),
```

### 10. Test Fixtures

**File:** `frontend/src/test-fixtures/session.ts`
- Add `DEFAULTS_BY_PROVIDER['opencode']` entry

---

## Database Migration

**New migration file:** `backend/migrations/YYYYMMDDHHMMSS_add_opencode_provider.up.sql`

```sql
-- New tokens_v2 card table
CREATE TABLE session_card_tokens_v2 (
    session_id    UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version       INTEGER NOT NULL DEFAULT 1,
    computed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    up_to_line    BIGINT NOT NULL DEFAULT 0,
    data          JSONB NOT NULL
);
```

---

## Docs

**New file:** `docs/src/content/docs/providers/opencode.md`
- OpenCode provider page: setup instructions, what data is collected, supported features

**Update:** `docs/src/content/docs/faq.md`
- Update FAQ answer from "Quite possibly" to "Yes, OpenCode is supported"

**Update:** Marketing copy locations (4 files)
- Replace "OpenCode next" with "OpenCode supported"

---

## Testing Strategy

### Backend

1. **Unit tests** for each analyzer (`analyzer_*_opencode_test.go`)
   - Test with fixture JSONL data covering all part types
   - Test edge cases: empty sessions, single-message sessions, multi-provider sessions
2. **Integration test** for the full pipeline: upload chunks → precompute → verify card data
3. **`TestRegistryCoversAllowedProviders`** automatically validates OpenCode registration
4. **Wire-level test**: verify `tokens_v2` card appears in API response with correct JSON shape
5. **Pricing tests**: verify cost calculation for Anthropic, OpenAI, and Google models through OpenCode

### Frontend

1. **`registry.test.ts`** drift guard validates adapter registration
2. **Storybook stories** for all new components (TokensV2Card, OpenCodeTranscriptPane, OpenCodeFilterDropdown)
3. **Component tests** for adapter methods (normalize, categorize, filter matching)
4. **E2E test** (if applicable): load an OpenCode session, verify summary panel renders

---

## File Manifest

### New Files (Backend)

| File | Purpose |
|------|---------|
| `backend/internal/analytics/opencode_provider.go` | SessionProvider implementation |
| `backend/internal/analytics/opencode_compute.go` | Orchestrator |
| `backend/internal/analytics/opencode_search.go` | Search index extraction |
| `backend/internal/analytics/analyzer_tokens_opencode.go` | Tokens analyzer |
| `backend/internal/analytics/analyzer_session_opencode.go` | Session analyzer |
| `backend/internal/analytics/analyzer_tools_opencode.go` | Tools analyzer |
| `backend/internal/analytics/analyzer_code_activity_opencode.go` | Code activity analyzer |
| `backend/internal/analytics/analyzer_conversation_opencode.go` | Conversation analyzer |
| `backend/internal/analytics/analyzer_agents_and_skills_opencode.go` | Agents & skills analyzer |
| `backend/internal/analytics/analyzer_redactions_opencode.go` | Redactions analyzer |
| `backend/internal/analytics/analyzer_smart_recap_opencode.go` | Smart recap transcript prep |
| `backend/migrations/YYYYMMDDHHMMSS_add_opencode_provider.up.sql` | DB migration |
| `backend/migrations/YYYYMMDDHHMMSS_add_opencode_provider.down.sql` | DB rollback |

### New Files (Frontend)

| File | Purpose |
|------|---------|
| `frontend/src/providers/opencodeAdapter.tsx` | Provider adapter |
| `frontend/src/components/session/OpenCodeTranscriptPane.tsx` | Transcript pane |
| `frontend/src/components/session/OpenCodeFilterDropdown.tsx` | Filter dropdown |
| `frontend/src/components/session/opencodeCategories.ts` | Categories + filter logic |
| `frontend/src/components/session/cards/TokensV2Card.tsx` | New tokens card component |
| `frontend/src/components/session/cards/TokensV2Card.stories.tsx` | Storybook stories |

### Modified Files (Backend)

| File | Change |
|------|--------|
| `backend/internal/models/provider.go` | Add `ProviderOpencode` constant |
| `backend/internal/pricingsource/pricing.json` | Add `"opencode"` provider block (major providers) |
| `backend/internal/analytics/compute_result.go` | Add `TokensV2Data` field |
| `backend/internal/analytics/cards.go` | Add `TokensV2CardRecord`, `TokensV2CardData` types |
| `backend/internal/analytics/store.go` | Add `session_card_tokens_v2` CRUD |
| `backend/internal/analytics/precompute.go` | Compute `tokens_v2` alongside `tokens` |
| `backend/internal/analytics/models.go` | Add `tokens_v2` to API response |

### Modified Files (Frontend)

| File | Change |
|------|--------|
| `frontend/src/utils/providers.ts` | Add `'opencode'` to PROVIDER_VALUES + metadata |
| `frontend/src/utils/tokenStats.ts` | Add `'opencode'` to default pricing |
| `frontend/src/providers/registry.ts` | Register OpenCode adapter |
| `frontend/src/providers/types.ts` | Add `OpenCodeAdapter` type alias |
| `frontend/src/components/icons.tsx` | Add `OpenCodeIcon` |
| `frontend/src/schemas/api.ts` | Add `TokensV2CardDataSchema`, update `AnalyticsCardsSchema` |
| `frontend/src/components/session/cards/registry.ts` | Add `tokens_v2` card definition |
| `frontend/src/test-fixtures/session.ts` | Add OpenCode defaults |

### Modified Files (Docs)

| File | Change |
|------|--------|
| `docs/src/content/docs/providers/opencode.md` | New provider page |
| `docs/src/content/docs/faq.md` | Update FAQ |
| `frontend/src/components/HeroCards.tsx` | Update marketing copy |
| `docs/src/content/docs/providers/claude-code.md` | Update "OpenCode next" |
| `docs/src/content/docs/providers/codex.md` | Update "OpenCode next" |
