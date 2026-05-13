// Zod schemas for Codex rollout JSONL validation.
//
// Each on-disk line has the envelope:
//   { timestamp: string, type: <top>, payload: object }
//
// Top-level `type` values:
//   session_meta            file header (model, cwd, git info)
//   turn_context            per-turn sandbox/approval policy
//   response_item           OpenAI Responses items (messages, function_call, ...)
//   event_msg               UI events (user_message, agent_message, token_count, ...)
//   compacted               context compaction replacement
//   (anything else)         caught by UnknownCodexLineSchema for forward-compat
//
// `response_item` and `event_msg` carry a nested discriminator `payload.type`.
// Each is defined as its own union of known shapes plus an Unknown catch-all,
// so unfamiliar future variants still parse cleanly.
//
// IMPORTANT: External data — use string() over enum() and .passthrough() everywhere.

import { z } from 'zod';

// ============================================================================
// response_item payload variants
// ============================================================================

const CodexInputTextSchema = z
  .object({
    type: z.literal('input_text'),
    text: z.string(),
  })
  .passthrough();

const CodexOutputTextSchema = z
  .object({
    type: z.literal('output_text'),
    text: z.string(),
  })
  .passthrough();

// Catch-all for unknown content-block types inside a message.
const CodexUnknownContentBlockSchema = z.object({ type: z.string() }).passthrough();

const CodexMessageContentSchema = z.union([
  CodexInputTextSchema,
  CodexOutputTextSchema,
  CodexUnknownContentBlockSchema,
]);

// response_item.message — both user and assistant flavors use this envelope.
// `role` is a string (not enum) for forward compat; renderers branch on its value.
const CodexResponseMessageSchema = z
  .object({
    type: z.literal('message'),
    role: z.string(), // 'user' | 'assistant' | 'developer' | future values
    content: z.array(CodexMessageContentSchema),
    // Assistant-only: `'commentary' | 'final'` (or future values). Optional
    // because user/developer messages don't carry it.
    phase: z.string().optional(),
  })
  .passthrough();

const CodexFunctionCallSchema = z
  .object({
    type: z.literal('function_call'),
    name: z.string(),
    arguments: z.string(), // JSON-stringified
    call_id: z.string(),
  })
  .passthrough();

const CodexFunctionCallOutputSchema = z
  .object({
    type: z.literal('function_call_output'),
    call_id: z.string(),
    output: z.string(),
  })
  .passthrough();

const CodexCustomToolCallSchema = z
  .object({
    type: z.literal('custom_tool_call'),
    call_id: z.string(),
    name: z.string(),
    input: z.string(),
    status: z.string().optional(),
  })
  .passthrough();

const CodexCustomToolCallOutputSchema = z
  .object({
    type: z.literal('custom_tool_call_output'),
    call_id: z.string(),
    output: z.string(),
  })
  .passthrough();

const CodexReasoningSchema = z
  .object({
    type: z.literal('reasoning'),
    summary: z.array(z.unknown()).optional(),
    content: z.unknown().nullable().optional(),
    encrypted_content: z.string().optional(),
  })
  .passthrough();

const CodexWebSearchActionSchema = z
  .object({
    type: z.string().optional(), // 'search' | future
    query: z.string().optional(),
    queries: z.array(z.string()).optional(),
  })
  .passthrough();

const CodexWebSearchCallSchema = z
  .object({
    type: z.literal('web_search_call'),
    status: z.string().optional(),
    action: CodexWebSearchActionSchema.optional(),
  })
  .passthrough();

// Catch-all for unknown response_item.payload.type variants.
const CodexUnknownResponseItemPayloadSchema = z
  .object({ type: z.string() })
  .passthrough();

const KnownResponseItemPayloadSchema = z.union([
  CodexResponseMessageSchema,
  CodexFunctionCallSchema,
  CodexFunctionCallOutputSchema,
  CodexCustomToolCallSchema,
  CodexCustomToolCallOutputSchema,
  CodexReasoningSchema,
  CodexWebSearchCallSchema,
]);

const CodexResponseItemPayloadSchema = z.union([
  KnownResponseItemPayloadSchema,
  CodexUnknownResponseItemPayloadSchema,
]);

export type KnownResponseItemPayload = z.infer<typeof KnownResponseItemPayloadSchema>;

const KNOWN_RESPONSE_ITEM_PAYLOAD_TYPES = new Set<string>([
  'message',
  'function_call',
  'function_call_output',
  'custom_tool_call',
  'custom_tool_call_output',
  'reasoning',
  'web_search_call',
]);

export function isKnownResponseItemPayload(
  p: { type: string },
): p is KnownResponseItemPayload {
  return KNOWN_RESPONSE_ITEM_PAYLOAD_TYPES.has(p.type);
}

// ============================================================================
// event_msg payload variants
// ============================================================================

const CodexEventUserMessageSchema = z
  .object({
    type: z.literal('user_message'),
    message: z.string(),
    images: z.array(z.unknown()).optional(),
  })
  .passthrough();

const CodexEventAgentMessageSchema = z
  .object({
    type: z.literal('agent_message'),
    message: z.string(),
    phase: z.string().optional(),
  })
  .passthrough();

const CodexEventTaskStartedSchema = z
  .object({
    type: z.literal('task_started'),
    turn_id: z.string().optional(),
    started_at: z.number().optional(),
    model: z.string().optional(),
    model_context_window: z.number().optional(),
  })
  .passthrough();

const CodexEventTaskCompleteSchema = z
  .object({
    type: z.literal('task_complete'),
    turn_id: z.string().optional(),
    last_agent_message: z.string().optional(),
    completed_at: z.number().optional(),
    duration_ms: z.number().optional(),
    time_to_first_token_ms: z.number().optional(),
  })
  .passthrough();

const CodexEventTokenCountSchema = z
  .object({
    type: z.literal('token_count'),
    info: z.unknown().nullable().optional(),
    rate_limits: z.unknown().optional(),
  })
  .passthrough();

const CodexPatchChangeSchema = z
  .object({
    type: z.string(), // 'add' | 'update' | 'delete' | future
    content: z.string().optional(),
  })
  .passthrough();

const CodexEventPatchApplyEndSchema = z
  .object({
    type: z.literal('patch_apply_end'),
    call_id: z.string().optional(),
    turn_id: z.string().optional(),
    stdout: z.string().optional(),
    stderr: z.string().optional(),
    success: z.boolean().optional(),
    changes: z.record(z.string(), CodexPatchChangeSchema).optional(),
  })
  .passthrough();

// Catch-all for unknown event_msg.payload.type variants.
const CodexUnknownEventPayloadSchema = z
  .object({ type: z.string() })
  .passthrough();

const KnownEventPayloadSchema = z.union([
  CodexEventUserMessageSchema,
  CodexEventAgentMessageSchema,
  CodexEventTaskStartedSchema,
  CodexEventTaskCompleteSchema,
  CodexEventTokenCountSchema,
  CodexEventPatchApplyEndSchema,
]);

const CodexEventPayloadSchema = z.union([
  KnownEventPayloadSchema,
  CodexUnknownEventPayloadSchema,
]);

export type KnownEventPayload = z.infer<typeof KnownEventPayloadSchema>;

const KNOWN_EVENT_PAYLOAD_TYPES = new Set<string>([
  'user_message',
  'agent_message',
  'task_started',
  'task_complete',
  'token_count',
  'patch_apply_end',
]);

export function isKnownEventPayload(
  p: { type: string },
): p is KnownEventPayload {
  return KNOWN_EVENT_PAYLOAD_TYPES.has(p.type);
}

// ============================================================================
// Top-level line envelopes
// ============================================================================

const CodexSessionMetaPayloadSchema = z
  .object({
    id: z.string().optional(),
    cwd: z.string().optional(),
    originator: z.string().optional(),
    model_provider: z.string().optional(),
    model: z.string().optional(),
  })
  .passthrough();

const CodexSessionMetaLineSchema = z
  .object({
    timestamp: z.string(),
    type: z.literal('session_meta'),
    payload: CodexSessionMetaPayloadSchema,
  })
  .passthrough();

const CodexTurnContextPayloadSchema = z
  .object({
    turn_id: z.string().optional(),
    cwd: z.string().optional(),
    approval_policy: z.string().optional(),
  })
  .passthrough();

const CodexTurnContextLineSchema = z
  .object({
    timestamp: z.string(),
    type: z.literal('turn_context'),
    payload: CodexTurnContextPayloadSchema,
  })
  .passthrough();

const CodexResponseItemLineSchema = z
  .object({
    timestamp: z.string(),
    type: z.literal('response_item'),
    payload: CodexResponseItemPayloadSchema,
  })
  .passthrough();

const CodexEventMsgLineSchema = z
  .object({
    timestamp: z.string(),
    type: z.literal('event_msg'),
    payload: CodexEventPayloadSchema,
  })
  .passthrough();

const CodexCompactedPayloadSchema = z
  .object({
    message: z.string().optional(),
    replacement_history: z.array(z.unknown()).optional(),
  })
  .passthrough();

const CodexCompactedLineSchema = z
  .object({
    timestamp: z.string(),
    type: z.literal('compacted'),
    payload: CodexCompactedPayloadSchema,
  })
  .passthrough();

// Catch-all for unknown top-level types.
const CodexUnknownLineSchema = z
  .object({
    timestamp: z.string(),
    type: z.string(),
    payload: z.unknown().optional(),
  })
  .passthrough();

/**
 * Union of the known top-level line types. Useful as the "narrowed" type
 * after `isKnownCodexLine` filters out the catch-all branch — at that point
 * each case in a `switch (line.type)` narrows to its dedicated branch.
 */
const KnownCodexLineSchema = z.union([
  CodexSessionMetaLineSchema,
  CodexTurnContextLineSchema,
  CodexResponseItemLineSchema,
  CodexEventMsgLineSchema,
  CodexCompactedLineSchema,
]);

export const RawCodexLineSchema = z.union([
  KnownCodexLineSchema,
  CodexUnknownLineSchema,
]);

export type RawCodexLine = z.infer<typeof RawCodexLineSchema>;
export type KnownCodexLine = z.infer<typeof KnownCodexLineSchema>;

const KNOWN_LINE_TYPES = new Set<string>([
  'session_meta',
  'turn_context',
  'response_item',
  'event_msg',
  'compacted',
]);

/**
 * Type predicate: narrows away the catch-all so a subsequent `switch`
 * on `line.type` discriminates cleanly between the 5 known branches.
 */
export function isKnownCodexLine(line: RawCodexLine): line is KnownCodexLine {
  return KNOWN_LINE_TYPES.has(line.type);
}

// ============================================================================
// Branch-level inferred types (used by the normalizer)
// ============================================================================

export type CodexSessionMetaLine = z.infer<typeof CodexSessionMetaLineSchema>;
export type CodexTurnContextLine = z.infer<typeof CodexTurnContextLineSchema>;
export type CodexResponseItemLine = z.infer<typeof CodexResponseItemLineSchema>;
export type CodexEventMsgLine = z.infer<typeof CodexEventMsgLineSchema>;
export type CodexCompactedLine = z.infer<typeof CodexCompactedLineSchema>;

export type CodexResponseMessage = z.infer<typeof CodexResponseMessageSchema>;
export type CodexFunctionCall = z.infer<typeof CodexFunctionCallSchema>;
export type CodexFunctionCallOutput = z.infer<typeof CodexFunctionCallOutputSchema>;
export type CodexCustomToolCall = z.infer<typeof CodexCustomToolCallSchema>;
export type CodexCustomToolCallOutput = z.infer<typeof CodexCustomToolCallOutputSchema>;
export type CodexWebSearchCall = z.infer<typeof CodexWebSearchCallSchema>;
export type CodexEventPatchApplyEnd = z.infer<typeof CodexEventPatchApplyEndSchema>;
export type CodexEventTaskComplete = z.infer<typeof CodexEventTaskCompleteSchema>;

// ============================================================================
// Parse-result types
// ============================================================================

// Re-export the existing validation-error shape so the Codex service doesn't
// invent a new one (it's a thin wrapper, same fields).
export type { TranscriptValidationError } from './transcript';

/**
 * Result of parsing a Codex rollout JSONL.
 */
export interface CodexParseResult {
  rawLines: RawCodexLine[];
  errors: import('./transcript').TranscriptValidationError[];
  totalLines: number;
  successCount: number;
  errorCount: number;
}
