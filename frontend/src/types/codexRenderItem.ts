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
 */
export interface CodexAssistantItem {
  kind: 'assistant';
  lineId: string;
  timestamp: CodexTimestamp;
  text: string;
  phase: 'commentary' | 'final';
  model: string;
  images?: string[];
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
  | CodexUnknownItem;
