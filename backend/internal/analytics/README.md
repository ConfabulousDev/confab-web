# analytics

Session analytics engine: parses Claude Code transcripts and computes, caches, and serves analytics cards.

## Files

| File | Role |
|------|------|
| `parser.go` | JSONL transcript line parser. Defines `TranscriptLine`, `MessageContent`, `TokenUsage`, `ContentBlock`, and helper predicates (`IsHumanMessage`, `GetToolUses`, etc.). |
| `file_collection.go` | `TranscriptFile` and `FileCollection` types. Parses raw JSONL bytes, validates lines, deduplicates assistant messages via `AssistantMessageGroups()`, and builds helper maps (timestamp, tool-use-ID-to-name). |
| `file_processor.go` | `FileProcessor` interface: the contract every analyzer implements (`ProcessFile` + `Finalize`). |
| `compute.go` | Orchestration layer. Defines the `AgentProvider` function type and `ComputeResult` aggregate struct. `ComputeStreaming` runs all eight analyzers through a three-phase pipeline (main file, streamed agents, finalize). Also provides `ComputeFromJSONL` and `ComputeFromFileCollection` convenience wrappers. |
| `analyzer_tokens.go` | `TokensAnalyzer` -- token counts and estimated cost via `pricing.go` functions. Falls back to `toolUseResult.usage` for agents without files. |
| `analyzer_session.go` | `SessionAnalyzer` -- message counts, message-type breakdown, duration, models used, compaction stats. |
| `analyzer_tools.go` | `ToolsAnalyzer` -- per-tool success/error counts. Attributes `tool_result` errors back to the originating tool via ID mapping. |
| `analyzer_code_activity.go` | `CodeActivityAnalyzer` -- files read/modified, lines added/removed, search count, language breakdown by extension. Inspects `Read`/`Write`/`Edit`/`Glob`/`Grep` tool inputs. |
| `analyzer_conversation.go` | `ConversationAnalyzer` -- user/assistant turn counts, turn timing, utilization percentage. Main-only (no agent files). |
| `analyzer_agents.go` | `AgentsAnalyzer` -- Task tool invocations grouped by `subagent_type`. Main-only. |
| `analyzer_skills.go` | `SkillsAnalyzer` -- Skill tool invocations plus command-expansion (`<command-name>`) detection. Main-only. |
| `analyzer_redactions.go` | `RedactionsAnalyzer` -- counts `[REDACTED:TYPE]` markers by recursively walking `RawData`. Processes all files. |
| `analyzer_smart_recap.go` | `SmartRecapAnalyzer` -- calls Anthropic LLM to generate session recaps. Handles transcript preparation (`PrepareTranscript`, `TranscriptBuilder`), stats formatting, response parsing, and message-ID resolution. Contains the system prompt. |
| `smart_recap_generator.go` | `SmartRecapGenerator` -- full lifecycle for smart recap: lock acquisition, LLM call, quota increment, card persistence, and suggested-title update. Used by both the precomputer and the on-demand API handler. |
| `agent_provider.go` | `AgentFileInfo`, `AgentDownloader`, and `NewAgentProvider()` -- streams agent files from storage one at a time, capping at `maxAgents` (0 = unlimited). |
| `cards.go` | Card record types (DB schema), card data types (API response), version constants, `IsValid`/`AllValid` staleness helpers. |
| `models.go` | `AnalyticsResponse` (API envelope), legacy flat types (`TokenStats`, `CostStats`, `CompactionInfo`). |
| `store.go` | `Store` -- DB CRUD for all card tables (`session_card_*`), search index, and smart recap. `GetCards`/`UpsertCards` run all queries in parallel. `ToCards` and `ToResponse` handle `ComputeResult <-> Cards <-> AnalyticsResponse` conversions. |
| `precompute.go` | `Precomputer` -- background worker entry points. `FindStaleSessions`, `PrecomputeRegularCards`, `FindStaleSmartRecapSessions`, `PrecomputeSmartRecapOnly`, `FindStaleSearchIndexSessions`, `BuildSearchIndexOnly`. |
| `search_index.go` | `SearchIndexContent`, `UserMessagesBuilder`, `ExtractSearchContent` -- builds weighted tsvector components (metadata=A, recap=B, user messages=C) for full-text search. |
| `pricing.go` | `ModelPricing`, `modelPricingTable`, `GetPricing`, `CalculateCost`, `CalculateTotalCost`. Per-model, per-million-token pricing with fast-mode and server-tool-use surcharges. |
| `validation.go` | Schema validation for every transcript line type (user, assistant, system, summary, file-history-snapshot, queue-operation, pr-link). |
| `trends.go` | `Store.GetTrends` -- single-user, date-range analytics dashboard. Runs five parallel aggregation queries (overview+activity, tokens, tools, agents+skills, top sessions). |
| `trends_types.go` | Request/response types for the trends API (`TrendsRequest`, `TrendsResponse`, `TrendsCards`, daily breakdown types). |
| `org_analytics.go` | `Store.GetOrgAnalytics` -- per-user aggregated analytics for admin org view. |
| `org_analytics_types.go` | Request/response types for org analytics (`OrgAnalyticsRequest`, `OrgAnalyticsResponse`, `OrgUserAnalytics`). |
| `utils.go` | `ExtractAgentID` -- extracts agent ID from filenames like `agent-{id}.jsonl`. |

## Key Types

### Card Records and Card Data

Each analytics card has a parallel pair of types:

- **`*CardRecord`** (e.g., `TokensCardRecord`) -- database row, includes `SessionID`, `Version`, `ComputedAt`, `UpToLine`, plus card-specific fields. Stored in `session_card_*` tables.
- **`*CardData`** (e.g., `TokensCardData`) -- API response payload, excludes DB metadata.

The `Cards` struct aggregates all seven regular card records and maps them to/from `AnalyticsResponse` via `ToCards()` and `ToResponse()`.

### FileProcessor Interface

```go
type FileProcessor interface {
    ProcessFile(file *TranscriptFile, isMain bool)
    Finalize(hasAgentFile func(string) bool)
}
```

Every analyzer implements this interface. `ProcessFile` is called once per transcript file (main first, then agents). `Finalize` is called after all files have been processed; the `hasAgentFile` callback lets analyzers decide whether to fall back to `toolUseResult` data for agents whose files were unavailable.

### Analyzer Result Types

Each analyzer has its own result struct (`TokensResult`, `SessionResult`, `ToolsResult`, etc.) returned by `Result()`. The `ComputeResult` struct in `compute.go` flattens all of these into a single aggregate.

### TranscriptFile and AssistantMessageGroup

`TranscriptFile` wraps parsed lines, an optional `AgentID`, and validation errors. Its `AssistantMessageGroups()` method deduplicates assistant messages that share the same `message.id` (multiple JSONL lines per API response, plus context replay). This is the canonical way to count tokens, models, and assistant responses without over-counting.

### Precomputer

`Precomputer` ties together storage, the analytics store, and configuration. It exposes three independent staleness-detection queries and their corresponding compute functions:

1. `FindStaleSessions` / `PrecomputeRegularCards` -- the seven deterministic cards
2. `FindStaleSmartRecapSessions` / `PrecomputeSmartRecapOnly` -- LLM-generated recap
3. `FindStaleSearchIndexSessions` / `BuildSearchIndexOnly` -- full-text search tsvector

### Store

`Store` wraps `*sql.DB` and provides get/upsert for every card table plus the search index. `GetCards` and `UpsertCards` fan out all queries in parallel.

## How to Extend

### Adding a New Analytics Card

Follow the `/add-session-card` skill (referenced in CLAUDE.md) for the full playbook. In summary:

1. **Version constant** -- add to `cards.go` (`FooCardVersion = 1`).
2. **Card record + card data types** -- add `FooCardRecord` (DB) and `FooCardData` (API) to `cards.go`.
3. **IsValid method** -- add to the record type; add the check to `Cards.AllValid`.
4. **Add field to `Cards` struct** -- e.g., `Foo *FooCardRecord`.
5. **Analyzer** -- create `analyzer_foo.go` implementing `FileProcessor`. Define `FooResult` and `Result()`.
6. **Register in `ComputeStreaming`** -- instantiate the analyzer and add it to the `processors` slice.
7. **Wire into `ComputeResult`** -- add fields, populate them from the analyzer result.
8. **`ToCards` / `ToResponse`** -- add conversion logic in `store.go`.
9. **Store operations** -- add `getFooCard` / `upsertFooCard` methods and wire them into `GetCards` / `UpsertCards`.
10. **Staleness queries** -- update `FindStaleSessions`, `FindStaleSmartRecapSessions`, and `FindStaleSearchIndexSessions` to JOIN the new `session_card_foo` table and check its version.
11. **DB migration** -- create the `session_card_foo` table.
12. **Frontend** -- add Zod schema, component, and registry entry.
13. **Tests** -- unit tests for the analyzer, integration tests for the store.

### Adding a New Analyzer (Without a Card)

If you need a new computation that feeds into an existing card (or is used only for internal purposes), implement `FileProcessor` and add it to the `processors` slice in `ComputeStreaming`. No DB or API changes are needed.

## Invariants

### Card Staleness

A card is **valid** when `Version == current constant` AND `UpToLine == session's current total line count`. The `IsValid` method on each record type encodes this. `Cards.AllValid` checks all seven regular cards.

Smart recap uses different staleness rules: `HasValidVersion()` checks only the version (time-based invalidation), while `IsUpToDate()` checks version and `UpToLine >= currentLineCount` (used by the precomputer).

### Version Bumping

Increment a card's version constant whenever compute logic changes. This triggers automatic recomputation via `FindStaleSessions`, which detects version mismatches. Existing cached data is overwritten on the next precompute cycle.

### Pricing Sync

The `modelPricingTable` in `pricing.go` must stay in sync with the frontend's `MODEL_PRICING` in `frontend/src/utils/tokenStats.ts`. When adding a new Anthropic model, update both.

### AgentProvider Contract

`AgentProvider` is `func(ctx context.Context) (*TranscriptFile, error)`. Return `io.EOF` when done. Errors on individual agent files are logged and skipped -- they must not fail the overall computation. The `Finalize` callback's `hasAgentFile` function reports which agents were actually processed, enabling fallback to `toolUseResult` data.

### Graceful Degradation

Individual card computation failures are collected in `CardErrors` rather than failing the entire pipeline. A card with an error is left `nil` in the `Cards` struct, and the error message is surfaced in the API response.

### Smart Recap Race Prevention

The smart recap uses an optimistic lock (`computing_started_at` column) to prevent concurrent LLM calls for the same session. `AcquireSmartRecapLock` atomically sets the timestamp; stale locks (older than `lockTimeoutSeconds`) are overridden. The lock is cleared on error or replaced by the upsert on success.

### Quota Enforcement

Smart recap quota is incremented BEFORE the card is saved. If quota tracking fails, the recap is discarded. This ensures usage is never under-counted.

## Design Decisions

### Card-per-Table Architecture

Each card type gets its own DB table (`session_card_tokens`, `session_card_session`, etc.) rather than a single wide table or a JSONB column. This allows:
- Independent schema evolution per card
- Parallel reads/writes (each card query is independent)
- Targeted indexing and query optimization
- Adding new cards without altering existing tables

### Precompute vs On-Demand

Regular cards are precomputed by a background worker that polls for stale sessions. This keeps API response latency low (just a DB read). The worker uses percentage-based staleness thresholds (`PrecomputeConfig.RegularCardsThresholds` and `SmartRecapThresholds`, both of type `StalenessThresholds`) with separate, more conservative thresholds for smart recap (which involves an LLM call).

The on-demand path (`ComputeStreaming`) is used by the API handler to compute fresh cards when the cache is stale and the user is actively viewing a session.

### Streaming Agent Processing

`ComputeStreaming` processes agent files one at a time through an `AgentProvider` function, keeping peak memory at `O(main) + O(largest single agent)` instead of `O(all agents)`. After each agent file is processed by all analyzers, it can be garbage collected.

### AssistantMessageGroups Deduplication

Claude Code transcripts emit one JSONL line per content block (text, tool_use, thinking) within a single API response. Context replay can repeat the same message later. `AssistantMessageGroups()` merges all lines sharing a `message.id` into one group. Token counts come from the last occurrence (final cumulative values), while boolean flags (hasText, hasToolUse, etc.) are OR-merged. This is the single source of truth for counting tokens and assistant responses.

### Search Index Weights

The full-text search index uses PostgreSQL tsvector weights:
- **Weight A** (highest): session metadata (titles, summary, first user message)
- **Weight B**: smart recap content
- **Weight C** (lowest): user messages from the transcript

This ensures title/summary matches rank higher than body text matches.

### Legacy Flat Format

`AnalyticsResponse` includes both legacy flat fields (`Tokens`, `Cost`, `Compaction`) and the new `Cards` map. This supports frontend migration; the flat fields will be removed once the frontend fully transitions to the cards format.

## Testing

- **Unit tests** (run with `go test -short`): analyzer logic, pricing calculations, parsing, validation, transcript preparation.
- **Integration tests** (require Docker via `testutil.SetupTestEnvironment`): store operations, precomputer staleness queries, smart recap end-to-end, search index, trends aggregation, org analytics.
- Test files follow the `*_test.go` convention alongside source files. `testdata_helpers_test.go` provides shared test fixtures.

Key test patterns:
- Analyzers are tested by constructing `TranscriptFile` or `FileCollection` objects with known lines and asserting on `Result()`.
- Store tests use containerized Postgres and verify round-trip get/upsert behavior.
- Precompute integration tests verify the full pipeline: insert test sessions, run `FindStaleSessions`, compute, and verify stored cards.

## Dependencies

### Imports (what this package uses)

| Dependency | Purpose |
|------------|---------|
| `github.com/shopspring/decimal` | Precise cost arithmetic (avoids floating-point rounding) |
| `go.opentelemetry.io/otel` | Distributed tracing spans on all Store and compute operations |
| `github.com/lib/pq` | PostgreSQL array parameters in trends queries |
| `github.com/ConfabulousDev/confab-web/internal/anthropic` | LLM client for smart recap generation |
| `github.com/ConfabulousDev/confab-web/internal/recapquota` | Monthly smart recap quota tracking |
| `github.com/ConfabulousDev/confab-web/internal/storage` | `DownloadAndMergeChunks` for transcript/agent file retrieval; `MaxAgentFiles` cap |

### Consumers (what imports this package)

| Consumer | Usage |
|----------|-------|
| `internal/api/analytics.go` | HTTP handler for session analytics (GET cards, trigger on-demand compute) |
| `internal/api/trends.go` | HTTP handler for the trends dashboard |
| `internal/api/org_analytics.go` | HTTP handler for admin org analytics |
| `cmd/server/worker.go` | Background worker that polls `FindStaleSessions` / `FindStaleSmartRecapSessions` / `FindStaleSearchIndexSessions` and calls the corresponding precompute functions |
