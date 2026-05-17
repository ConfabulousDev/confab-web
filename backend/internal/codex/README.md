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
| `parser.go` | `ParseRollout(io.Reader) (*ParsedRollout, error)` plus the streaming state machine, line dispatch, tool-call pairing, and exec_command output-preamble parsing. |
| `types.go` | `ParsedRollout`, `Turn`, `Message`, `ToolCall`, `TokenUsage`, `CompactionEvent`, `ValidationError`. Pure data types — no imports beyond `time`. |
| `parser_test.go` | Unit tests against `testdata/sample_rollout.jsonl`. Covers the 10 spec scenarios (happy path, mid-turn truncation, bad JSON, empty input, null token info, exec exit codes, env-context stripping, compaction, orphan output, implicit turn). |
| `testdata/sample_rollout.jsonl` | Fixture covering session_meta, three completed turns (turns 1–2 are the standard happy paths; turn 3 carries an inline-failed `custom_tool_call` per CF-438), function_call + custom_tool_call, web_search_call, encrypted reasoning, non-null `token_count.info`, a compacted line, an unknown top-level type (forward-compat), and a trailing orphan `function_call_output`. |

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
9. **Developer-role messages**: dropped (not analytics-relevant).
10. **`<environment_context>…</environment_context>`**: stripped from user
    messages (greedy regex). If stripping leaves an empty string, the
    message is dropped.

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
| `internal/analytics/codex_adapter.go` | `ComputeFromCodexRollout(*ParsedRollout)` maps the rollout onto `ComputeResult` for card upsert. |
| `internal/analytics/codex_search.go` | `ExtractCodexUserMessagesText` flattens user / assistant-final / tool-call text into the search index Weight C content. |
| `internal/analytics/codex_transcript.go` | `PrepareCodexTranscript` builds the XML transcript fed to the smart recap LLM. |
| `internal/analytics/precompute.go` | `precomputeRegularCardsCodex`, `buildSearchIndexCodex`, `precomputeSmartRecapCodex` — the three top-level branches dispatched when `StaleSession.Provider == "codex"`. |
