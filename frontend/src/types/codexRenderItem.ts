// Render-time types for the Codex transcript view.
//
// The Codex rollout JSONL is rich and partially redundant (event_msg events
// often mirror response_item events). `normalizeCodexLines` collapses that
// stream into the items below, which the timeline renders one row each.

/** ISO 8601 timestamp string, sourced from the originating JSONL line. */
export type CodexTimestamp = string;

/** User prompt — derived from `response_item.message[role=user]`. */
export interface CodexUserItem {
  kind: 'user';
  timestamp: CodexTimestamp;
  text: string;
}

/**
 * Assistant text — derived from `response_item.message[role=assistant]`.
 * `phase: 'commentary'` indicates interim narration; `'final'` is the answer
 * the user is expected to read.
 */
export interface CodexAssistantItem {
  kind: 'assistant';
  timestamp: CodexTimestamp;
  text: string;
  phase: 'commentary' | 'final';
  model: string;
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
  timestamp: CodexTimestamp;
}

/**
 * Turn boundary emitted on `event_msg.task_complete`.
 * `turnIndex` is computed during normalization (1-based).
 */
export interface CodexTurnSeparatorItem {
  kind: 'turn_separator';
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
