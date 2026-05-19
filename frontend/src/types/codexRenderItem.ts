// Render-time types for the Codex transcript view.
//
// The Codex rollout JSONL is rich and partially redundant (event_msg events
// often mirror response_item events). `normalizeCodexLines` collapses that
// stream into the items below, which the timeline renders one row each.
//
// ## `lineId` (CF-360)
//
// Every variant carries a stable string identifier so the UI can deep-link to
// individual rows via `?msg=<lineId>`. The value is `String(idx)` where `idx`
// is the position of the originating line in the validated `rawLines` array
// passed to `normalizeCodexLines()`. It is:
//
//   - Stable across re-renders of the same `rawLines` (the array is
//     append-only across polling cycles).
//   - Monotonic — earlier source lines get smaller numeric ids.
//   - Unique across emitted items (each render item ties back to exactly
//     one *initial* source line; output-pairing lines mutate the existing
//     item and do NOT mint a new id).
//
// NOT the literal source-file line number. `parseCodexJSONL` drops empty and
// schema-invalid lines, so the rawLines index differs from the on-disk row.
// If a future feature needs the literal file line number, add a separate
// `originalLineNumber` on `RawCodexLine` rather than redefining `lineId`.
//
// The string type is intentional so a future swap (content-hash, server-issued
// UUID) doesn't break the prop shape.

import type { TokenUsage } from '@/utils/tokenStats';

/** ISO 8601 timestamp string, sourced from the originating JSONL line. */
export type CodexTimestamp = string;

/**
 * User prompt — derived from `response_item.message[role=user]`.
 *
 * `images` (CF-388) carries any `input_image.image_url` values from the same
 * message in document order. Omitted on text-only items so their shape is
 * byte-identical to pre-CF-388 output.
 */
export interface CodexUserItem {
  kind: 'user';
  lineId: string;
  timestamp: CodexTimestamp;
  text: string;
  images?: string[];
}

/**
 * Assistant text — derived from `response_item.message[role=assistant]`.
 * `phase: 'commentary'` indicates interim narration; `'final'` is the answer
 * the user is expected to read.
 *
 * `images` (CF-388) carries any `output_image.image_url` values from the same
 * message in document order. Omitted on text-only items.
 *
 * `usage` (CF-362, CF-418) is attached after-the-fact when an
 * `event_msg.token_count` line is processed — the most-recent unannotated
 * assistant item (any phase) gets the `last_token_usage` delta, normalized
 * to canonical `TokenUsage` at parse time (uncached input, output with
 * reasoning folded in, cacheRead = cached_input_tokens, cacheWrite = 0).
 *
 * `reasoningTokens` (CF-418) preserves the raw reasoning count so the cost
 * tooltip can show a "Reasoning: N" sub-line even though `usage.output`
 * already includes it for billing.
 *
 * Tool calls never carry usage because the model API attributes one call's
 * cost to the response group as a whole.
 */
export interface CodexAssistantItem {
  kind: 'assistant';
  lineId: string;
  timestamp: CodexTimestamp;
  text: string;
  phase: 'commentary' | 'final';
  model: string;
  images?: string[];
  usage?: TokenUsage;
  reasoningTokens?: number;
}

/**
 * A paired tool call + output. Codex emits these as siblings keyed by
 * `call_id`; the normalizer pairs them into a single item.
 *
 * `status: 'pending'` means the matching `function_call_output` /
 * `custom_tool_call_output` has not arrived yet (in-flight session).
 *
 * `structuredOutput` carries provider-specific structured info that is more
 * useful than the raw `output` string (e.g. `apply_patch.changes` from
 * `event_msg.patch_apply_end`). Both can coexist; both render side by side.
 */
export interface CodexToolCallItem {
  kind: 'tool_call';
  /**
   * The `rawLines` index of the line that *created* the call
   * (function_call / custom_tool_call / web_search_call). Subsequent output
   * lines mutate this item in-place and do not change `lineId`.
   */
  lineId: string;
  timestamp: CodexTimestamp;
  toolName: string;
  callId: string;
  rawInput: unknown;
  rawOutput?: string;
  structuredOutput?: unknown;
  status: 'pending' | 'completed' | 'failed' | 'unknown';
  /** For `exec_command`: parsed from the `Chunk ID: …` preamble. */
  execMetadata?: { exitCode: number; wallTimeMs: number };
  /**
   * CF-368: present iff a paired `event_msg.mcp_tool_call_end` enriched this
   * call. Carries the MCP server and tool names so the renderer can label
   * the row `<server> / <tool>` instead of the bare function name. Only set
   * when at least one of `server` / `tool` is non-empty.
   */
  mcpInvocation?: { server: string; tool: string };
}

/**
 * Placeholder for an encrypted `reasoning` line. Content is opaque so the
 * UI shows a small "(reasoning hidden)" marker rather than rendering raw JSON.
 */
export interface CodexReasoningHiddenItem {
  kind: 'reasoning_hidden';
  lineId: string;
  timestamp: CodexTimestamp;
}

/**
 * Turn boundary emitted on `event_msg.task_complete`.
 * `turnIndex` is computed during normalization (1-based).
 */
export interface CodexTurnSeparatorItem {
  kind: 'turn_separator';
  lineId: string;
  timestamp: CodexTimestamp;
  turnIndex: number;
  durationMs: number;
  timeToFirstTokenMs?: number;
}

/**
 * Emitted on `compacted` lines: a context compaction event replaced N prior
 * messages with a summary.
 */
export interface CodexCompactedItem {
  kind: 'compacted';
  lineId: string;
  timestamp: CodexTimestamp;
  replacementCount: number;
}

/**
 * CF-368: divider for an aborted turn (`event_msg.turn_aborted`). Fires
 * when the user interrupts the model mid-turn (Esc / kill), the turn is
 * replaced, a review ended, or the budget cap was hit. The Codex upstream
 * enum lists those four reasons (`interrupted` | `replaced` | `review_ended`
 * | `budget_limited`); we store the raw string for forward-compat with new
 * variants. Empty string when the field was missing on the wire.
 */
export interface CodexTurnAbortedItem {
  kind: 'turn_aborted';
  lineId: string;
  timestamp: CodexTimestamp;
  /** Wire `reason`, snake_case. Empty string when absent. */
  reason: string;
  /** Wall-clock duration of the aborted turn. 0 when absent. */
  durationMs: number;
}

/**
 * Forward-compat fallback. Any line whose top-level `type` (or nested
 * `payload.type`) is unrecognized lands here so the timeline still renders
 * something useful instead of crashing or silently dropping content.
 */
export interface CodexUnknownItem {
  kind: 'unknown';
  lineId: string;
  timestamp: CodexTimestamp;
  rawLine: unknown;
}

/** Discriminated union over `kind`. */
export type CodexRenderItem =
  | CodexUserItem
  | CodexAssistantItem
  | CodexToolCallItem
  | CodexReasoningHiddenItem
  | CodexTurnSeparatorItem
  | CodexCompactedItem
  | CodexTurnAbortedItem
  | CodexUnknownItem;
