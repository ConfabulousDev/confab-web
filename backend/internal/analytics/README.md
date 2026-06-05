# analytics

Session analytics engine: parses Claude Code and Codex transcripts, then computes, caches, and serves analytics cards.

## Per-card / per-provider naming convention (CF-441, CF-454)

Per-card compute logic lives in `analyzer_<card>_<provider>.go` files. One file per (card, provider) pair makes the matrix scannable — `ls analyzer_*.go` shows the full grid at a glance:

| Card | Claude file | Codex file |
|---|---|---|
| Tokens | `analyzer_tokens_claude.go` | `analyzer_tokens_codex.go` |
| Session | `analyzer_session_claude.go` | `analyzer_session_codex.go` |
| Tools | `analyzer_tools_claude.go` | `analyzer_tools_codex.go` |
| Code Activity | `analyzer_code_activity_claude.go` | `analyzer_code_activity_codex.go` |
| Conversation | `analyzer_conversation_claude.go` | `analyzer_conversation_codex.go` |
| Agents and Skills | `analyzer_agents_and_skills_claude.go` (two `FileProcessor`s — `AgentsAnalyzer` and `SkillsAnalyzer` — feeding one combined card) | `analyzer_agents_and_skills_codex.go` (CF-443: `spawn_agent` → AgentStats keyed by `agent_role`; `<skill>` blocks → SkillStats keyed by skill name) |
| Redactions | `analyzer_redactions_claude.go` | `analyzer_redactions_codex.go` |
| Workflows | `analyzer_workflows.go` (CF-534: per-run subagent aggregates; Claude-only, driven explicitly by `ComputeStreaming`, not a `FileProcessor`) | — (Codex has no workflows) |
| Smart Recap | `analyzer_smart_recap.go` (shared infrastructure: LLM call, prompt assembly, response parsing, `FormatConfig`) + `analyzer_smart_recap_claude.go` (Claude transcript prep: `PrepareTranscript`, `TranscriptBuilder`) | `analyzer_smart_recap_codex.go` (`PrepareCodexTranscript`) |

The orchestrators follow the same convention: `claude_compute.go` ↔ `codex_compute.go`, with the shared `ComputeResult` aggregate living in `compute_result.go` (CF-454).

## Files

| File | Role |
|------|------|
| `parser.go` | JSONL transcript line parser. Defines `TranscriptLine`, `MessageContent`, `TokenUsage`, `ContentBlock`, and helper predicates (`IsHumanMessage`, `GetToolUses`, etc.). |
| `file_collection.go` | `TranscriptFile` and `FileCollection` types. Parses raw JSONL bytes, validates lines, deduplicates assistant messages via `AssistantMessageGroups()`, and builds helper maps (timestamp, tool-use-ID-to-name). |
| `file_processor.go` | `FileProcessor` interface: the contract every Claude-side analyzer implements (`ProcessFile` + `Finalize`). |
| `claude_compute.go` | Orchestration layer for Claude. Defines the `AgentProvider` function type. `ComputeStreaming` runs all eight Claude analyzers through a three-phase pipeline (main file, streamed agents, finalize). Also provides `ComputeFromJSONL` and `ComputeFromFileCollection` convenience wrappers. |
| `compute_result.go` | `ComputeResult` — the provider-agnostic aggregate produced by both `ComputeStreaming` (Claude) and `ComputeFromCodexRollout` (Codex), then mapped onto per-card DB records by `store.go`. |
| `analyzer_tokens_claude.go` | `TokensAnalyzer` — token counts and estimated cost via `pricing.go` functions. Falls back to `toolUseResult.usage` for agents without files. |
| `analyzer_session_claude.go` | `SessionAnalyzer` — message counts, message-type breakdown, duration, models used, compaction stats. |
| `analyzer_tools_claude.go` | `ToolsAnalyzer` — per-tool success/error counts. Attributes `tool_result` errors back to the originating tool via ID mapping. |
| `analyzer_code_activity_claude.go` | `CodeActivityAnalyzer` — files read/modified, lines added/removed, search count, language breakdown by extension. Inspects `Read`/`Write`/`Edit`/`Glob`/`Grep` tool inputs. |
| `analyzer_conversation_claude.go` | `ConversationAnalyzer` — user/assistant turn counts, turn timing, utilization percentage. Main-only (no agent files). |
| `analyzer_agents_and_skills_claude.go` | `AgentsAnalyzer` (Agent/Task tool invocations grouped by `subagent_type`) and `SkillsAnalyzer` (Skill tool invocations plus command-expansion `<command-name>` detection). Two `FileProcessor`s, main-only, feeding the combined Agents & Skills card (CF-454). |
| `analyzer_redactions_claude.go` | `RedactionsAnalyzer` — counts `[REDACTED:TYPE]` markers by recursively walking `RawData`. Processes all files. |
| `analyzer_workflows.go` | `WorkflowsAnalyzer` (CF-534) — per-run workflow subagent aggregates (agent count, token breakdown + cost, journal-derived success count, activity span). Driven explicitly by `ComputeStreaming` via `ProcessAgent`/`ProcessJournal`/`Result` (not a `FileProcessor`). Claude-only. |
| `analyzer_smart_recap.go` | `SmartRecapAnalyzer` — calls Anthropic LLM to generate session recaps. Shared infrastructure: LLM call, `PrepareStats`, response parsing (`parseSmartRecapResponse`, `resolveMessageIDs`), system-prompt sections + `BuildSmartRecapSystemPrompt`, and the `FormatConfig` truncation helper used by both providers' transcript-prep paths. |
| `analyzer_smart_recap_claude.go` | Claude transcript prep for smart recap: `PrepareTranscript`, `PrepareTranscriptFromFiles`, `TranscriptBuilder` + `NewTranscriptBuilder`, and the `formatLine` / `formatUserLine` / `formatAssistantLine` helpers that emit `<user>` / `<assistant>` / `<skill>` / `<tool_results>` XML from `TranscriptLine`s. |
| `smart_recap_generator.go` | `SmartRecapGenerator` — full lifecycle for smart recap: lock acquisition, LLM call, quota increment, card persistence, and suggested-title update. Resolves custom system prompt from `dbadminsettings` at generation time. Used by both the precomputer and the on-demand API handler. |
| `agent_provider.go` | `AgentFileInfo`, `AgentDownloader`, and `NewAgentProvider()` — streams agent files from storage one at a time, capping at `maxAgents` (0 = unlimited). |
| `cards.go` | Card record types (DB schema), card data types (API response), version constants, `IsValid`/`AllValid` staleness helpers. |
| `models.go` | `AnalyticsResponse` (API envelope), legacy flat types (`TokenStats`, `CostStats`, `CompactionInfo`). |
| `store.go` | `Store` — DB CRUD for all card tables (`session_card_*`), search index, and smart recap. `GetCards`/`UpsertCards` run all queries in parallel. `ToCards` and `ToResponse` handle `ComputeResult <-> Cards <-> AnalyticsResponse` conversions. |
| `precompute.go` | `Precomputer` — background worker entry points. `FindStaleSessions`, `PrecomputeRegularCards`, `FindStaleSmartRecapSessions`, `PrecomputeSmartRecapOnly`, `FindStaleSearchIndexSessions`, `BuildSearchIndexOnly`. Stale-session filters cover all analytics-eligible providers via `models.AllowedProviders`; the three top-level compute methods dispatch through `ProviderFor(StaleSession.Provider)`. |
| `provider.go` | `SessionProvider`, `ParseInput`, `RegisterProvider`, `ProviderFor` — registry contract. Providers register a canonical name plus aliases at init time; unknown providers return loud errors. `SessionProvider.DisplayName()` returns the human-facing label (used by `email/email.go` for share invitation subjects). |
| `claude_provider.go` | `claudeProvider` — Claude-Code implementation of `SessionProvider`. Registers canonical `claude-code` plus legacy `Claude Code`. `claudeRollout` caches parsed agent files on `cachedAgents` after the first traversal, so subsequent calls to `ComputeCards`, `SearchText`, and `PrepareTranscript` on the same rollout instance reuse them without a second S3 download. |
| `codex_provider.go` | `codexProvider` — Codex implementation of `SessionProvider`. Registers `codex`. `codexRollout.materialize` discovers subagent rollout files via `sync_files` (`file_type='agent'`, capped at `storage.MaxAgentFiles`), downloads + parses each on first use, caches the result, and prefixes their `ValidationError` reasons with the file name. Per-subagent failures log and append a synthetic `ValidationError` to main but never abort the rollout. |
| `codex_compute.go` | `ComputeFromCodexRollout([]*codex.ParsedRollout)` — orchestrator. Tokens and Session aggregate across the full slice internally; Conversation reads `rollouts[0]` only (per-card asymmetry); Tools / CodeActivity / AgentsSkills / Redactions are dispatched per-rollout and accumulate via `+=`. `ValidationErrorCount` sums across rollouts so the frontend counter reflects the union. |
| `analyzer_tokens_codex.go` | `computeCodexTokens` — OpenAI-aware token math (cached tokens subset of input, no cache-write charge, reasoning tokens are a subset of `output_tokens` on the wire and pass through unchanged, CF-471). |
| `analyzer_session_codex.go` | `computeCodexSession` — message counts, breakdown, models used, duration, compactions (all classified as "auto"; Codex doesn't distinguish auto vs manual). `HumanPrompts == UserMessages` for Codex by construction: the parser separates tool outputs (`function_call_output` / `custom_tool_call_output` → `turn.ToolCalls`) from user messages at the wire format, so no `IsHumanMessage`-style filter is needed at compute time. See `analyzer_session_codex_test.go` for the regression guard. |
| `analyzer_tools_codex.go` | `computeCodexTools` — per-tool success/error breakdown. Inline-failed `custom_tool_call` payloads (Status `"failed"`) increment per-tool `Errors` and `ToolErrorCount`. Orphan `<unknown>` synthesized tools are dropped from the per-tool breakdown and excluded from `TotalToolCalls` / `ToolErrorCount`; the anomaly surfaces via `ParsedRollout.ValidationErrors` instead. CF-438. `spawn_agent` and `wait_agent` calls are routed out of `Turn.ToolCalls` by the parser (CF-443) so they don't appear here. |
| `analyzer_code_activity_codex.go` | `computeCodexCodeActivity` — apply_patch envelope parsing for `FilesModified` / `LinesAdded` / `LinesRemoved` / `LanguageBreakdown`. `FilesRead` stays 0 (Codex has no Read tool). `SearchCount` stays 0 — `web_search_call` is web search, not file search (CF-439). |
| `analyzer_conversation_codex.go` | `computeCodexConversation` — UserTurns / AssistantTurns plus the five timing fields (CF-441). Flattens all message events across turns and walks them in timestamp order, mirroring Claude's `analyzer_conversation_claude.go` semantics. Reasoning extends the assistant window via a synthetic event at `Turn.CompletedAt` (Codex-specific divergence documented inline). |
| `analyzer_agents_and_skills_codex.go` | `computeCodexAgentsAndSkills` — populates `AgentStats` from `ParsedRollout.SubagentSpawns` (success iff `wait_agent` reported `"completed"`, else error — including orphan spawns) bucketed by `agent_role`; populates `SkillStats` from `ParsedRollout.SkillInvocations` bucketed by skill name (always success — Codex emits no per-skill error signal). CF-443. |
| `analyzer_redactions_codex.go` | `computeCodexRedactions` — walks parser-surfaced strings for `[REDACTED:TYPE]` markers. Uses the same `redactionPattern` and TYPE-placeholder exclusion as the Claude path. Note (CF-445): relies on the Confab CLI redacting at upload time. |
| `codex_search.go` | `ExtractCodexUserMessagesText([]*codex.ParsedRollout)` -- flattens user messages, assistant `final` text, and tool-call summaries across main + subagent rollouts into the Weight C search-index content. Honors the 500 KB byte cap (applied to the combined output) with UTF-8-safe boundary alignment. (Codex-only; the Claude equivalent is inlined in `claude_provider.go`. Deliberate asymmetry — no Claude counterpart yet.) |
| `analyzer_smart_recap_codex.go` | `PrepareCodexTranscript([]*codex.ParsedRollout)` -- builds the XML transcript fed to the smart recap LLM. Main turns first, then each subagent's turns + compactions inline (no `<subagent>` wrapper), mirroring Claude's per-file `TranscriptBuilder.ProcessFile` pattern. Codex synthesizes ids the frontend doesn't anchor on; `codexProvider.ClearMessageIDs()` requests post-LLM zeroing. |
| `search_index.go` | `SearchIndexContent`, `UserMessagesBuilder`, `ExtractSearchContent` -- builds weighted tsvector components (metadata=A, recap=B, user messages=C) for full-text search. |
| `pricing.go` | `ModelPricing`, `GetPricing`, `CalculateCost`, `CalculateTotalCost`, `SetActivePricing`. Per-model, per-million-token pricing with fast-mode and server-tool-use surcharges. The active table is an `atomic.Pointer` seeded from `pricingsource.Embedded()` and swapped by the worker via `SetActivePricing(pricingsource.Effective(...))`. |
| `validation.go` | Schema validation for every transcript line type (user, assistant, system, summary, file-history-snapshot, queue-operation, pr-link). |
| `trends.go` | `Store.GetTrends` -- date-range analytics dashboard for sessions visible to the caller (visibility model identical to `/api/v1/sessions`). Runs seven parallel aggregation queries (overview+activity, tokens, tools, agents+skills, top sessions, providers-present, filter-options). Every aggregation routes through one `buildTrendsQuery` prelude that wraps `db.VisibleSessionsCTE` + a shared `filtered_sessions` CTE, so the visibility predicate and `?owner=` narrowing live in exactly one place (CF-495). `aggregateFilterOptions` is the only path that bypasses `filtered_sessions` — it derives owners + repos from `visible_sessions` directly so the dropdown is static across active filter changes (mirrors `SessionFilterOptions`). The overview+activity path groups by `(session_date, session_type)` so `DailySessionCount.PerProvider` carries per-canonical-provider counts for the stacked-bar chart (CF-444); legacy `Claude Code` folds into `claude-code` at the Scan site. `resolveProviderFilter` expands canonical provider values with legacy aliases and defaults to `models.AllowedProviders` so the `session_type = ANY` clause is always present (guards CF-352-style silent omission). |
| `trends_types.go` | Request/response types for the trends API (`TrendsRequest` with `Providers` + `Owners` + `ShareAllSessions` (CF-495), `TrendsResponse` with top-level `ProvidersPresent` + `FilterOptions`, `TrendsCards`, daily breakdown types, plus `TrendsTokensPerProvider` + `TrendsTokensCard.PerProvider` map for CF-435 and `DailySessionCount.PerProvider` map for CF-444). |
| `org_analytics.go` | `Store.GetOrgAnalytics` -- per-user aggregated analytics for the admin Org view. Supports `Providers` (canonical filter via `resolveProviderFilter`, shared with trends) and `Repos` / `IncludeNoRepo` (mirrors the trends repo predicate). Emits `ProvidersPresent` from a separate DISTINCT-by-session_type query; legacy `Claude Code` rows fold into `claude-code` via `models.NormalizeProvider`. |
| `org_analytics_types.go` | Request/response types for org analytics (`OrgAnalyticsRequest` carries `Providers`/`Repos`/`IncludeNoRepo`; `OrgAnalyticsResponse` exposes `ProvidersPresent` plus the renamed `TotalAssistantTimeMs`/`AvgAssistantTimeMs` fields). |
| `utils.go` | `ExtractAgentID` -- extracts agent ID from filenames like `agent-{id}.jsonl` (matched on the path basename, so nested workflow paths resolve too). `ExtractWorkflowRunID` -- extracts `<runId>` from `subagents/workflows/<runId>/...`. |

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

Each analyzer has its own result struct (`TokensResult`, `SessionResult`, `ToolsResult`, etc.) returned by `Result()`. The `ComputeResult` struct in `compute_result.go` flattens all of these into a single aggregate.

### TranscriptFile and AssistantMessageGroup

`TranscriptFile` wraps parsed lines, an optional `AgentID`, and validation errors. Its `AssistantMessageGroups()` method deduplicates assistant messages that share the same `message.id` (multiple JSONL lines per API response, plus context replay). This is the canonical way to count tokens, models, and assistant responses without over-counting.

### Precomputer

`Precomputer` ties together storage, the analytics store, and configuration. It exposes three independent staleness-detection queries and their corresponding compute functions:

1. `FindStaleSessions` / `PrecomputeRegularCards` -- the seven deterministic cards
2. `FindStaleSmartRecapSessions` / `PrecomputeSmartRecapOnly` -- LLM-generated recap (four staleness categories: missing, version mismatch, threshold-based, and admin-triggered regeneration)
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

Smart recap uses different staleness rules: `HasValidVersion()` checks only the version (time-based invalidation), while `IsUpToDate()` checks version and `UpToLine >= currentLineCount` (used by the precomputer). A fourth staleness category (admin-triggered regeneration) marks cards as stale when `computed_at < regen_requested_at` from the `admin_settings` table; this is indicated by a non-nil `RegenRequestedAt` field on `StaleSession`.

### Version Bumping

Increment a card's version constant whenever compute logic changes. This triggers automatic recomputation via `FindStaleSessions`, which detects version mismatches. Existing cached data is overwritten on the next precompute cycle.

### Pricing source

Prices are no longer hardcoded here. The single source is `internal/pricingsource/pricing.json` (embedded there). `GetPricing` reads an active table that `init` seeds from `pricingsource.Embedded()` and the precompute worker refreshes each cycle via `SetActivePricing(pricingsource.Effective(ctx))` — so a self-hosted backend picks up new prices pulled from confabulous.dev without a redeploy. To change a price, edit `pricing.json` and bump `updated_at`.

### AgentProvider Contract

`AgentProvider` is `func(ctx context.Context) (*TranscriptFile, error)`. Return `io.EOF` when done. Errors on individual agent files are logged and skipped -- they must not fail the overall computation. The `Finalize` callback's `hasAgentFile` function reports which agents were actually processed, enabling fallback to `toolUseResult` data.

### Graceful Degradation

Individual card computation failures are collected in `CardErrors` rather than failing the entire pipeline. A card with an error is left `nil` in the `Cards` struct, and the error message is surfaced in the API response.

### Smart Recap Race Prevention

The smart recap uses an optimistic lock (`computing_started_at` column) to prevent concurrent LLM calls for the same session. `AcquireSmartRecapLock` atomically sets the timestamp; stale locks (older than `lockTimeoutSeconds`) are overridden. The lock is cleared on error or replaced by the upsert on success.

### Quota Enforcement

Smart recap quota is incremented BEFORE the card is saved. If quota tracking fails, the recap is discarded. This ensures usage is never under-counted. Admin-triggered bulk regeneration (category 4 staleness, indicated by `StaleSession.RegenRequestedAt != nil`) bypasses quota checks entirely. The per-session `admin_card_invalidations` audit table (CF-343) also bypasses quota: when a row exists for the session with `session_card_smart_recap` in `card_types` and the recap has not been recomputed past `invalidated_at`, the session qualifies regardless of quota.

### Customizable System Prompt

The smart recap system prompt is composed of four sections: input format, output schema, instructions, and example. The instructions section is customizable by admins via the `admin_settings` table (key: `smart_recap_system_prompt`). The other three sections are fixed in code. `BuildSmartRecapSystemPrompt(instructions *string)` assembles the full prompt:
- `nil` instructions: use the hardcoded default (`DefaultSmartRecapInstructions()`)
- Empty string: omit the instructions section entirely
- Non-empty string: use the custom instructions

`SmartRecapFixedSections()` returns the three fixed sections (input format, output schema, example) for the admin API to display as read-only context.

**Provider-agnostic by design (CF-447).** The same prompt is sent for every coding-agent provider (Claude Code, Codex, and any future addition). The input format describes the transcript structure categorically (user messages, assistant responses, tool calls, tool results, compaction markers) without naming agent-specific tags like `<skill>` or `<thinking>`. The example output and the output schema reference "the project's agent config file" rather than a specific filename like `CLAUDE.md` or `AGENTS.md`. Adding a new provider does NOT require touching the prompt.

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

### Provider Registry (Claude vs Codex)

The precompute worker (`PrecomputeRegularCards`, `BuildSearchIndexOnly`, `PrecomputeSmartRecapOnly`) AND the on-demand API handler (`api/analytics.go::HandleGetSessionAnalytics`, `HandleRegenerateSmartRecap`) both resolve the provider with `ProviderFor` and dispatch through the `SessionProvider` interface (CF-402, CF-403):

- `Parse(ctx, ParseInput) (Rollout, error)` loads provider-specific session data and returns nil for empty sessions.
- `ComputeCards(ctx, Rollout) *ComputeResult` maps the provider rollout to the canonical card aggregate.
- `SearchText(ctx, Rollout) string` returns Weight C transcript text for search indexing.
- `PrepareTranscript(ctx, Rollout) (string, map[int]string, error)` builds smart recap XML and the message-id map.
- `ClearMessageIDs() bool` reports whether smart recap annotations should drop frontend anchors.
- `DisplayName() string` returns the human-facing label (e.g. "Claude Code", "Codex"); concatenated with " session" by `email/email.go::humanProviderLabel`.

Providers register at init time with `RegisterProvider`. Claude registers both canonical `claude-code` and legacy `Claude Code` to the same provider instance; Codex registers `codex`. Duplicate registrations panic during startup/tests. Unknown providers return a loud `unsupported provider for analytics` error.

**Lazy-materialize convention.** Each provider's `Rollout` caches files parsed during the first traversal. `claudeRollout.cachedAgents` populates inside the agent provider streaming pass; `codexRollout.cachedAgents` populates inside `materialize`. Subsequent calls to `ComputeCards`, `SearchText`, and `PrepareTranscript` on the same rollout instance replay the cache without re-downloading. This contract is single-goroutine — rollouts are not safe for concurrent use.

**Codex subagent aggregation (CF-403).** `codexProvider.Parse` discovers `sync_files` rows with `file_type IN ('transcript','agent')`. The main transcript is loaded eagerly; subagent files (capped at `storage.MaxAgentFiles`) are lazily materialized on first analytics method call. Per-file download / parse failures log a warning and append a synthetic `ValidationError` to main's slice, but never abort the rollout. The codex orchestrator (`ComputeFromCodexRollout`) aggregates across the full slice; the Conversation card alone stays main-only by design (UserTurns / AssistantTurns + timing reflect user-perceived structure, not subagent reasoning).

**Workflow subagent attribution (CF-532).** Claude **workflow** subagents (the `Workflow` tool) upload their transcripts under a path-encoded `file_name`: `subagents/workflows/<runId>/agent-<id>.jsonl` (`file_type=agent`) plus an append-only `subagents/workflows/<runId>/journal.jsonl` (`file_type=workflow_journal`). Because `ExtractAgentID` matches on the path basename, `downloadClaudeMainAndListAgents` picks these up and their tokens flow through `TokensAnalyzer.ProcessFile(agent)` like any other agent file. There is no double counting: the main transcript's `Workflow` tool_result is `status:"async_launched"` with no `agentId`/`usage`, so the `Finalize` fallback never fires for it. `ExtractWorkflowRunID` recovers the run grouping from the path (no DB column). The path convention is a load-bearing contract written verbatim by the CLI (CF-533).

**Workflows card (CF-534).** The same workflow files also back a dedicated per-run **Workflows card** (`session_card_workflows`, JSONB `runs`). `WorkflowsAnalyzer` (`analyzer_workflows.go`) is driven **explicitly** by `ComputeStreaming` — it is NOT a `FileProcessor` and is not in the `processors` slice, because a run needs a runId per agent (resolved via `WorkflowInputs.RunIDByAgentID`, built in `claudeRollout.buildWorkflowInputs` from agent file names) plus the run journal, neither of which the generic loop models. `downloadClaudeMainAndListAgents` now also selects `workflow_journal` files; `ComputeCards` downloads each journal and passes them in `WorkflowInputs.Journals` (keyed by runId). `ProcessAgent` accumulates per-run agent count, token breakdown + cost (mirroring `TokensAnalyzer`), and a first→last activity span; `ProcessJournal` derives `SucceededAgents` from `result`-line presence (the locked CF-533 schema carries no explicit status — only-`started` agents are indistinguishably errored-or-running). The card is **always written** (empty `runs` for non-workflow sessions) so it participates in the `FindStaleSessions` all-cards-exist gate like the other seven; it is hidden on the frontend when empty. Run timing's `StartedAt` is `json:"-"` (compute-time ordering only). Journal lines are excluded from `total_lines`, so journal-only growth does not trigger recompute (status may lag until the next agent-file change or a version bump).

The three stale-session SQL filters use `WHERE session_type = ANY($N)` with `pq.Array(models.AllowedProviders)` (canonical + legacy aliases) and each query returns `session_type AS provider`, normalized through `models.NormalizeProvider` at the Scan site. `TestRegistryCoversAllowedProviders` guarantees that every value in `AllowedProviders` resolves through the registry. `TestPrecomputeGoHasNoProviderSwitchOrLiterals` (in this package) and `TestAnalyticsGoHasNoProviderLiterals` (in `internal/api/`) source-scan the two dispatch boundaries for provider literals and helpers, guarding against regressions.

Per-card mapping decisions for Codex are documented inline in `codex_compute.go`. Notable points:

- OpenAI `cached_input_tokens` is a SUBSET of `input_tokens` (unlike Anthropic). The adapter subtracts it before applying the uncached input rate; OpenAI cache writes are free (`CacheWrite=0` across all gpt-/o-series entries in `pricing.go`).
- OpenAI `reasoning_output_tokens` is also a SUBSET of `output_tokens` on the wire (CF-471). `computeCodexTokens` passes `output_tokens` through unchanged — reasoning is informational, never additive — so the wire identity `total_tokens = (input - cached) + cached + output` is preserved. The raw reasoning count is preserved on the assistant render item for the cost-tooltip sub-line.
- `FilesRead` stays 0 — Codex has no Read tool; documented inline rather than approximated heuristically.
- `AssistantTurns` count user-prompt-triggered sequences (not raw Codex `task_started`->`task_complete` cycles) for closer parity with Claude semantics.
- Smart recap items get empty `MessageID` when the provider reports `ClearMessageIDs() == true`; the frontend's `SmartRecapCard.tsx` already short-circuits on `!item.message_id` and renders them as plain text (Codex rollout JSONL has no stable per-message id for deep-linking — the synthetic `codex-msg-N`/`codex-tool-N` ids in `PrepareCodexTranscript`'s idMap exist only for the LLM's internal cross-references and are zeroed before the card is saved). See the header godoc on `analyzer_smart_recap_codex.go::PrepareCodexTranscript` for the full rationale (CF-447).

### Adding a New Provider

See `PROVIDER_EXTENSION.md` (in this directory) for the full checklist.

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
| `github.com/ConfabulousDev/confab-web/internal/db/dbadminsettings` | Custom smart recap prompt retrieval |
| `github.com/ConfabulousDev/confab-web/internal/recapquota` | Monthly smart recap quota tracking |
| `github.com/ConfabulousDev/confab-web/internal/storage` | `DownloadAndMergeChunks` for transcript/agent file retrieval; `MaxAgentFiles` cap |

### Consumers (what imports this package)

| Consumer | Usage |
|----------|-------|
| `internal/api/analytics.go` | HTTP handler for session analytics (GET cards, trigger on-demand compute) |
| `internal/api/trends.go` | HTTP handler for the trends dashboard |
| `internal/api/org_analytics.go` | HTTP handler for admin org analytics |
| `internal/admin/api_handlers.go` | Smart recap prompt settings endpoints (reads `DefaultSmartRecapInstructions`, `SmartRecapFixedSections`) |
| `cmd/server/worker.go` | Background worker that polls `FindStaleSessions` / `FindStaleSmartRecapSessions` / `FindStaleSearchIndexSessions` and calls the corresponding precompute functions |
