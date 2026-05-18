# codex

Parser for OpenAI Codex CLI rollout JSONL files.

## Purpose

Codex sessions are uploaded to Confab as JSONL rollouts emitted by the Codex
CLI. This package normalizes those files into a structured representation
(`ParsedRollout`) that the `analytics` package consumes for cards, smart
recap, and the search index — the analogue of how `analytics.parser` parses
Claude Code transcripts.

The frontend has its own Codex parser (`frontend/src/services/codexTranscriptService.ts`)
for transcript rendering. The Go parser here is independent and serves the
backend pipelines.

## Files

| File | Role |
|------|------|
| `parser.go` | `ParseRollout(io.Reader) (*ParsedRollout, error)` plus the streaming state machine, line dispatch, tool-call pairing, exec_command output-preamble parsing, subagent spawn/wait routing, and skill / `<skills_instructions>` / `<subagent_notification>` extraction. |
| `types.go` | `ParsedRollout`, `Turn`, `Message`, `ToolCall`, `TokenUsage`, `CompactionEvent`, `ValidationError`, plus `SubagentSource`, `SkillInvocation`, `SubagentSpawn`, `SkillAvailable` (CF-443). Pure data types — no imports beyond `time`. |
| `parser_test.go` | Unit tests against the fixtures below. Covers the legacy parser scenarios plus CF-443 (session_meta source variants, `<skills_instructions>` catalog, `<skill>` invocation extraction + stripping, `spawn_agent` / `wait_agent` routing, `<subagent_notification>` stripping, depth>1, completion-text truncation). |
| `testdata/sample_rollout.jsonl` | Legacy fixture: session_meta, three completed turns (turn 3 carries an inline-failed `custom_tool_call` per CF-438), function_call + custom_tool_call, web_search_call, encrypted reasoning, non-null `token_count.info`, a compacted line, an unknown top-level type (forward-compat), and a trailing orphan `function_call_output`. |
| `testdata/sample_rollout_with_skill_invocation.jsonl` | CF-443: developer message with `<skills_instructions>` catalog plus a user message wrapping a single `<skill>` invocation. |
| `testdata/sample_rollout_parent_with_spawns.jsonl` | CF-443: parent-side rollout with two `spawn_agent` calls (one completed, one failed) and a `wait_agent` reporting both outcomes, plus a `<subagent_notification>` user-message artifact. |

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

## Invariants

- `ParseRollout` returns `(*ParsedRollout, error)` — error is only set for
  stream-level reader failures. Per-line decode failures appear in
  `ValidationErrors`.
- `ParsedRollout.Turns` may end with an open turn (no `CompletedAt`) when
  the rollout was truncated mid-stream.
- `ToolCall.Status` is one of `"pending"`, `"completed"`, `"failed"`.
- `Message.Phase` is `"final"` for assistant messages with no explicit phase
  and is empty for user messages.

## Dependencies

| Dependency | Purpose |
|------------|---------|
| stdlib only (`bufio`, `bytes`, `encoding/json`, `regexp`, `strconv`, `strings`, `time`, `io`, `fmt`) | The parser intentionally avoids project imports so it can be used as a leaf package by `analytics` without cycles. |

## Consumers

| Consumer | Usage |
|----------|-------|
| `internal/analytics/codex_compute.go` | `ComputeFromCodexRollout([]*ParsedRollout)` maps the rollout slice (main + subagents) onto `ComputeResult` for card upsert. |
| `internal/analytics/codex_search.go` | `ExtractCodexUserMessagesText([]*ParsedRollout)` flattens user / assistant-final / tool-call text across all rollouts into the search index Weight C content. |
| `internal/analytics/analyzer_smart_recap_codex.go` | `PrepareCodexTranscript([]*ParsedRollout)` builds the XML transcript fed to the smart recap LLM (main turns first, then each subagent's turns inline). |
| `internal/analytics/codex_provider.go` | `codexProvider.Parse` → `codexRollout.materialize` discovers subagent rollouts via `sync_files` (`file_type='agent'`), downloads + parses each on first use (`codex.ParseRollout` once per file), caches the result, and prefixes their `ValidationError` reasons with the file name. |
| `internal/analytics/precompute.go` + `internal/api/analytics.go` | Both dispatch through `analytics.ProviderFor("codex")` → `codexProvider` (CF-402, CF-403). No provider literals at the dispatch boundary. |
