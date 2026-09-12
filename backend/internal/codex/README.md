# codex

Parser for OpenAI Codex CLI rollout JSONL files.

## Purpose

Codex sessions are uploaded to Confab as JSONL rollouts emitted by the Codex
CLI. This package normalizes those files into a structured representation
(`ParsedRollout`) that the `analytics` package consumes for cards, smart
recap, and the search index — the analogue of how `analytics.parser` parses
Claude Code transcripts.

The package also exposes a standalone user-message extractor (`firstuser.go`)
for callers that need only the human prompt off a single line, without parsing
the whole rollout — the sync ingest path uses it to derive `first_user_message`.

The frontend has its own Codex parser (`frontend/src/services/codexTranscriptService.ts`)
for transcript rendering. The Go parser here is independent and serves the
backend pipelines.

## Files

| File | Role |
|------|------|
| `parser.go` | `ParseRollout(io.Reader) (*ParsedRollout, error)` plus the streaming state machine, line dispatch, tool-call pairing, exec_command output-preamble parsing, subagent spawn/wait routing, skill / `<skills_instructions>` / `<subagent_notification>` extraction, and the >=0.149.1 `item_completed` item dispatch (`handleItemCompleted`). |
| `types.go` | `ParsedRollout`, `Turn`, `Message`, `ToolCall`, `TokenUsage`, `CompactionEvent`, `ValidationError`, plus `SubagentSource`, `SkillInvocation`, `SubagentSpawn`, `SkillAvailable` (CF-443) and `FileEdit` (m2ky). Pure data types — no imports beyond `time`. |
| `firstuser.go` | `EventMsgUserText(json.RawMessage) string` and `UserMessageFromLine(string) string` — extract the human-typed prompt from an `event_msg` line across both wire eras (`user_message` on <=0.130.0, `item_completed` → `UserMessage` on >=0.149.1). Reads the event_msg stream only; the user-role `response_item` stream is injected context (`<environment_context>`, AGENTS.md), never the prompt. Returns `""` for anything underivable. |
| `firstuser_test.go` | Unit tests for the two extractors against wire shapes captured from real rollouts: both eras, mixed `text`/`skill` content parts, and the response_item / non-UserMessage lines that must yield `""`. |
| `itemcompleted_test.go` | Unit tests for the >=0.149.1 `item_completed` dispatch (m2ky): FileChange → sorted `FileEdits`, CommandExecution → `ParsedCommandKinds`, Extension `web.search` → synthesized tool call, unobserved Extension kinds and unhandled item types skipped, UserMessage/AgentMessage deliberately not double-recorded, a mixed-era rollout, and malformed items. Fixture lines reproduce real captured wire shapes. |
| `parser_test.go` | Unit tests against the fixtures below. Covers the legacy parser scenarios plus CF-443 (session_meta source variants, `<skills_instructions>` catalog, `<skill>` invocation extraction + stripping, `spawn_agent` / `wait_agent` routing, `<subagent_notification>` stripping, depth>1, completion-text truncation). |
| `testdata/sample_rollout.jsonl` | Legacy fixture: session_meta, three completed turns (turn 3 carries an inline-failed `custom_tool_call` per CF-438), function_call + custom_tool_call, web_search_call, encrypted reasoning, non-null `token_count.info`, a compacted line, an unknown top-level type (forward-compat), and a trailing orphan `function_call_output`. |
| `testdata/sample_rollout_with_skill_invocation.jsonl` | CF-443: developer message with `<skills_instructions>` catalog plus a user message wrapping a single `<skill>` invocation. |
| `testdata/sample_rollout_parent_with_spawns.jsonl` | CF-443: parent-side rollout with two `spawn_agent` calls (one completed, one failed) and a `wait_agent` reporting both outcomes, plus a `<subagent_notification>` user-message artifact. |
| `testdata/sample_rollout_modern.jsonl` | m2ky: a >=0.149.1-era rollout — session_meta without `model`, `item_completed` items (UserMessage, Reasoning, two CommandExecutions with `parsed_cmd`, an `Extension` web.search, a FileChange with one `update` and one `add`, AgentMessage), plus the response_item twins (`custom_tool_call` name `exec` whose `input` is JavaScript, and the user/assistant messages) that prove the two streams both carry the conversation. |

## Parser contract

`ParseRollout` consumes a Reader, scans line-by-line with a 4 MB token cap,
and applies the following rules:

1. **JSON-decode failures**: skip the line, record a `ValidationError`,
   continue. ParseRollout never returns an error for malformed individual
   lines — only for stream-level errors.
2. **Top-level types**: `session_meta`, `turn_context`, `response_item`,
   `event_msg`, `compacted`. Unknown top-level types are skipped silently
   (forward-compat); they are NOT recorded as errors. `turn_context` is
   inspected only for its `model` field — Codex CLI ~0.130+ moved `model`
   out of `session_meta` into `turn_context`, so the first `turn_context`
   fills session-level `Model` when `session_meta.model` is absent, and
   fills `Turn.Model` when `task_started` carried no model.
3. **Turn boundaries**: a new turn begins on `event_msg.task_started` (or
   implicitly on the first response_item if no task_started has fired).
   Closes on `event_msg.task_complete`. Files ending mid-turn leave the last
   turn open with nil `CompletedAt`/`DurationMs`.
4. **Tool-call pairing**: each `call_id` is indexed; `function_call` and
   `custom_tool_call` create the entry; `*_output` populates `Output` and
   `Status`; `event_msg.patch_apply_end` overrides `Status` to `"failed"`
   when `success: false`. A `custom_tool_call` carrying `status: "completed"`
   or `status: "failed"` inline (e.g. `apply_patch` reporting failure on the
   call rather than via a later `patch_apply_end`) propagates that status
   onto the open ToolCall immediately; unknown statuses fall through to
   `"pending"` for a later `*_output` to resolve (CF-438). **Orphan outputs**
   (`function_call_output` with no matching `function_call`) create a
   synthetic ToolCall named `"<unknown>"` in an implicit turn so transcript
   and search still surface the output text, and append a `ValidationError`
   per occurrence so downstream consumers can detect the anomaly.
5. **exec_command output preamble** (`Chunk ID:`, `Wall time:`, `Process
   exited with code N`, `Output:\n`): parsed into `ExitCode`, `WallTimeMs`,
   and the body. Mirrors the frontend's `parseExecOutput`.
6. **`compacted`**: append a `CompactionEvent`; do NOT drop prior turns.
   The rollout file is the source of truth; `replacement_history` is for
   CLI resume, not analytics.
7. **Encrypted reasoning**: increment `ReasoningCount` (per turn); no
   displayable text is recorded.
8. **TokenUsage**: updated from every `event_msg.token_count` event whose
   `info` is non-null. Final state is the last non-null
   `info.total_token_usage`. CachedInputTokens is a SUBSET of InputTokens
   (OpenAI semantics) — callers that bill cached tokens separately must
   subtract before applying the uncached rate.
9. **Developer-role messages**: dropped after first scanning for a
   `<skills_instructions>` block. The first such block populates
   `AvailableSkills` (parsed from `### Available skills` bullets); subsequent
   blocks are ignored.
10. **`<environment_context>`, `<skill>`, `<subagent_notification>`**:
    stripped from user messages (non-greedy regex). `<skill>` blocks
    additionally append a `SkillInvocation`. `<subagent_notification>` is
    stripped only — the structured data is sourced from `wait_agent` output
    instead. If stripping leaves an empty string, the message is dropped.
11. **`spawn_agent` / `wait_agent` function_calls** (CF-443): routed out of
    `Turn.ToolCalls`. `spawn_agent` appends a `SubagentSpawn` with
    `agent_type`, `message`, `reasoning_effort`, `fork_context`; its
    `function_call_output` fills `ResultAgentID` + `ResultNickname` and
    indexes the spawn by agent_id. A later `wait_agent` output (matched via
    that index) sets `Completed` (true iff status key is `"completed"`),
    `CompletionStatus`, and `CompletionText` (truncated to 1000 chars).
    Orphan spawns (no `wait_agent` in the rollout) remain `Completed=false`.

12. **`event_msg.item_completed`** (>=0.149.1): dispatched by `item.type`.
    `CommandExecution` appends each `parsed_cmd[].type` to
    `Turn.ParsedCommandKinds`; `FileChange` flattens `item.changes` into
    `Turn.FileEdits`, **sorted by path** so downstream accumulation never
    depends on JSON map order; `Extension` with `kind: "web.search"`
    synthesizes a `ToolCall` named `web.search` (mirroring the <=0.130.0
    `web_search_call` synthesis). `UserMessage`, `AgentMessage`, `Reasoning`,
    `ImageView` and unknown item types are deliberately **not** recorded — see
    "Wire-format eras" below. Items are never recorded as tool calls apart from
    that one synthesis.
13. **`handleEventMsg`'s top-level `default:` branch** (px58): every other
    `event_msg.payload.type` the parser sees falls here and is silently
    ignored — no crash, no analytics impact. Mirrors the frontend's
    `KNOWN_EVENT_PAYLOAD_TYPES` recognize-but-silent set: redundant with the
    response_item stream (`user_message`, `agent_message`,
    `agent_reasoning`, `agent_reasoning_raw_content`); redundant with the
    top-level `compacted` line (`context_compacted`); redundant with a
    call_id-paired response_item already resolved above (`web_search_end`,
    `mcp_tool_call_end`); or internal thread bookkeeping with no analytics
    use today (`thread_settings_applied`, `thread_goal_updated`,
    `thread_rolled_back`, `sub_agent_activity`, `entered_review_mode`,
    `exited_review_mode`, `image_generation_end`). Anything not on this list
    also lands here — Codex's own `should_persist_event_msg` rollout policy
    says nothing else should ever reach a rollout file, so an unrecognized
    arrival here is a forward-compat signal, not a bug.

## Wire-format eras

Codex reshaped its rollout format at **0.149.1** (~2026-08-25). Old rollouts
live in the object store forever, so both shapes are supported permanently; the
pre-0.149 shape is frozen, so this does not grow over time. Handling is
**per-event, with no rollout-level era switch** — a session started before a CLI
upgrade and resumed after it contains both shapes in one file.

What moved:

| Signal | <=0.130.0 | >=0.149.1 |
|--------|-----------|-----------|
| File edits | `apply_patch` custom_tool_call, `*** Begin Patch` envelope | `item_completed` → `FileChange`, per-path map of `{type, unified_diff}` (update) or `{type, content}` (add) |
| Shell runs | `exec_command` function_call, arguments are JSON | `exec` custom_tool_call whose `input` is **JavaScript**; the structured form (`command`, `cwd`, `parsed_cmd`, `status`) is on `item_completed` → `CommandExecution` |
| Web search | `web_search_call` response_item + `web_search_end` event | `item_completed` → `Extension`, `kind: "web.search"` |
| User / assistant text | `event_msg.user_message` / `agent_message`, plus response_item `message` | `item_completed` → `UserMessage` / `AgentMessage`, plus response_item `message` |
| Patch status | `event_msg.patch_apply_end` | carried inline on the `FileChange` item |
| `model` | `session_meta.model` | absent from session_meta; `turn_context` / `task_started` only |

**The two streams inverted, and cannot be merged.** For a modern shell run the
response_item carries JavaScript (`const r = await tools.exec_command({cmd:"…"})`)
while the structured data is on the event side. Their ids never overlap
(`call_…` vs `exec-…`) and their counts diverge (one sampled rollout: 122
`custom_tool_call` vs 42 `CommandExecution`), so there is no dedupe-on-merge
design available. Each consumer reads whichever stream carries its signal.

**Why UserMessage / AgentMessage items are dropped.** Modern rollouts carry the
conversation in *both* streams — verified across every captured >=0.149.1
rollout — and the parser already reads `response_item`, so recording the event
copy would double every message count. The one consumer that needs the
event-stream user text is server-side `first_user_message` derivation, which
runs before parsing and reads it through `EventMsgUserText` in `firstuser.go`
— the single home for that shape.

### Present in modern rollouts, deliberately unconsumed

- **`token_usage_record`** (new top-level type, 380 occurrences across the
  sampled rollouts). Per-response token accounting, richer than
  `event_msg.token_count`:
  `payload.{thread_id, turn_id, session_id, root_turn_id, response_id}` plus
  three identical-shaped blocks — `usage`, `turn_token_usage` and
  `thread_token_usage` — each
  `{input_tokens, cached_input_tokens, cache_write_input_tokens, output_tokens, reasoning_output_tokens, total_tokens}`.
  Token analytics are **not** broken (`token_count` survives in modern
  rollouts), so adopting this is a feature, not a fix — it is the per-turn
  substrate ticket 90c3 needs.
- **`world_state`** (carries `agents_md` text) and **`item.ImageView`** — no
  analyzer consumes either.

### Open question

`compacted` / `context_compacted` appear in **no** captured >=0.149.1 rollout.
Either the representation changed again or none of the sampled sessions
compacted. There is no modern sample to design against, so `handleCompacted` is
left keyed on the old shape rather than guessed at. Modern compaction handling
is unverified.

## Invariants

- `ParseRollout` returns `(*ParsedRollout, error)` — error is only set for
  stream-level reader failures. Per-line decode failures appear in
  `ValidationErrors`.
- `ParsedRollout.Turns` may end with an open turn (no `CompletedAt`) when
  the rollout was truncated mid-stream.
- `ToolCall.Status` is one of `"pending"`, `"completed"`, `"failed"`.
- `Message.Phase` is `"final"` for assistant messages with no explicit phase
  and is empty for user messages.
- `Turn.FileEdits` is sorted by `Path`. It and `Turn.ParsedCommandKinds` are
  always empty for <=0.130.0 rollouts.
- `ParsedCommandKinds` values are Codex's own open-ended vocabulary. Consumers
  must bucket recognized kinds explicitly and treat an unrecognized kind as
  uncounted — `"unknown"` is the majority of real entries.

## Dependencies

| Dependency | Purpose |
|------------|---------|
| stdlib only (`bufio`, `bytes`, `encoding/json`, `regexp`, `sort`, `strconv`, `strings`, `time`, `io`, `fmt`) | The parser intentionally avoids project imports so it can be used as a leaf package by `analytics` without cycles. |

## Consumers

| Consumer | Usage |
|----------|-------|
| `internal/api/sync.go` | `codex.UserMessageFromLine` derives `first_user_message` from the lines of a codex transcript chunk 1 when the CLI omitted the metadata field. Codex sessions have no summary, so a NULL `first_user_message` makes the session invisible in the list and in Trends (tgxn). |
| `internal/analytics/codex_compute.go` | `ComputeFromCodexRollout([]*ParsedRollout)` maps the rollout slice (main + subagents) onto `ComputeResult` for card upsert. |
| `internal/analytics/codex_search.go` | `ExtractCodexUserMessagesText([]*ParsedRollout)` flattens user / assistant-final / tool-call text across all rollouts into the search index Weight C content. |
| `internal/analytics/analyzer_smart_recap_codex.go` | `PrepareCodexTranscript([]*ParsedRollout)` builds the XML transcript fed to the smart recap LLM (main turns first, then each subagent's turns inline). |
| `internal/analytics/codex_provider.go` | `codexProvider.Parse` → `codexRollout.materialize` discovers subagent rollouts via `sync_files` (`file_type='agent'`), downloads + parses each on first use (`codex.ParseRollout` once per file), caches the result, and prefixes their `ValidationError` reasons with the file name. |
| `internal/analytics/precompute.go` + `internal/api/analytics.go` | Both dispatch through `analytics.ProviderFor("codex")` → `codexProvider` (CF-402, CF-403). No provider literals at the dispatch boundary. |
