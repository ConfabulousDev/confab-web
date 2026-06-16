// Zod schemas for Claude Code transcript validation
// Validates external transcript data at parse time to ensure type safety
//
// IMPORTANT: These schemas validate external data we don't control.
// Use string() instead of enum() for fields that could have new values added.
// Use passthrough() to preserve unknown fields for forward compatibility.
//
// Reference: the open-source claude-code-log parser
// (https://github.com/daaain/claude-code-log) maintains an independent catalog
// of Claude Code JSONL line types and shapes — a useful cross-check when adding
// or extending schemas here, or when triaging new metadata line types.
import { z } from 'zod';

// ============================================================================
// Content Block Schemas
// ============================================================================

const TextBlockSchema = z.object({
  type: z.literal('text'),
  text: z.string(),
});

const ThinkingBlockSchema = z.object({
  type: z.literal('thinking'),
  thinking: z.string(),
  signature: z.string().optional(), // Optional for forward compat
});

const ToolUseBlockSchema = z.object({
  type: z.literal('tool_use'),
  id: z.string(),
  name: z.string(),
  input: z.record(z.string(), z.unknown()),
});

// ToolResultBlock can contain nested content blocks or a string
const ToolResultBlockSchema = z.object({
  type: z.literal('tool_result'),
  tool_use_id: z.string(),
  content: z.union([z.string(), z.array(z.lazy(() => ContentBlockSchema))]),
  is_error: z.boolean().optional(),
});

const ImageBlockSchema = z.object({
  type: z.literal('image'),
  source: z.object({
    type: z.string(), // 'base64' | 'url' - use string for forward compat
    media_type: z.string(),
    data: z.string().optional(),
    url: z.string().optional(),
  }),
});

const ToolReferenceBlockSchema = z.object({
  type: z.literal('tool_reference'),
  tool_name: z.string(),
});

// Catch-all for forward compatibility — must be last in the union.
// Unknown block types pass validation and render with a fallback UI.
const UnknownBlockSchema = z.object({ type: z.string() }).passthrough();

const ContentBlockSchema: z.ZodType<ContentBlock> = z.union([
  TextBlockSchema,
  ThinkingBlockSchema,
  ToolUseBlockSchema,
  ToolResultBlockSchema,
  ImageBlockSchema,
  ToolReferenceBlockSchema,
  UnknownBlockSchema,
]);

// Infer types from schemas
export type TextBlock = z.infer<typeof TextBlockSchema>;
export type ThinkingBlock = z.infer<typeof ThinkingBlockSchema>;
export type ToolUseBlock = z.infer<typeof ToolUseBlockSchema>;
export type ImageBlock = z.infer<typeof ImageBlockSchema>;
export type ToolReferenceBlock = z.infer<typeof ToolReferenceBlockSchema>;

// ContentBlock needs manual definition due to recursion
export type UnknownBlock = { type: string; [key: string]: unknown };

export type ContentBlock =
  | TextBlock
  | ThinkingBlock
  | ToolUseBlock
  | { type: 'tool_result'; tool_use_id: string; content: string | ContentBlock[]; is_error?: boolean }
  | ImageBlock
  | ToolReferenceBlock
  | UnknownBlock;

// ============================================================================
// Token Usage Schema
// ============================================================================

const ServerToolUseSchema = z.object({
  web_search_requests: z.number().optional(),
  web_fetch_requests: z.number().optional(),
  code_execution_requests: z.number().optional(),
}).passthrough(); // forward-compat for new tool types

const TokenUsageSchema = z.object({
  input_tokens: z.number(),
  cache_creation_input_tokens: z.number().optional(),
  cache_read_input_tokens: z.number().optional(),
  cache_creation: z
    .object({
      ephemeral_5m_input_tokens: z.number(),
      ephemeral_1h_input_tokens: z.number(),
    })
    .optional(),
  output_tokens: z.number(),
  service_tier: z.string().nullable().optional(),
  server_tool_use: ServerToolUseSchema.optional(),
  speed: z.string().optional(),
  inference_geo: z.string().optional(),
  iterations: z.array(z.unknown()).optional(),
}).passthrough(); // forward-compat for future usage fields

// CF-418: canonical TokenUsage shape stamped by the parse layer onto
// assistant messages after wire validation. Not a wire field.
const NormalizedTokenUsageSchema = z.object({
  input: z.number(),
  output: z.number(),
  cacheWrite: z.number(),
  cacheWrite1h: z.number(),
  cacheRead: z.number(),
});

// ============================================================================
// Message Schemas
// ============================================================================

const BaseMessageSchema = z.object({
  uuid: z.string(),
  timestamp: z.string(),
  parentUuid: z.string().nullable(),
  isSidechain: z.boolean(),
  userType: z.string(),
  cwd: z.string(),
  sessionId: z.string(),
  version: z.string(),
  gitBranch: z.string().optional(),
});

const ThinkingMetadataSchema = z.object({
  level: z.string().optional(), // 'high' | 'medium' | 'low' | 'off' - use string for forward compat
  disabled: z.boolean().optional(),
  triggers: z.array(z.string()).optional(),
  maxThinkingTokens: z.number().optional(),
}).passthrough();

// Bash tool result (Claude Code >= 2.1.143). All fields optional: interrupted/
// isImage/noOutputExpected ride on every recent Bash result; returnCodeInterpretation
// and persistedOutput* appear only when their condition is met. `.passthrough()`
// keeps any sibling fields (e.g. backgroundTaskId) that we don't model yet.
export const BashToolResultSchema = z.object({
  stdout: z.string().optional(),
  stderr: z.string().optional(),
  exitCode: z.number().nullable().optional(),
  interrupted: z.boolean().optional(),
  isImage: z.boolean().optional(),
  noOutputExpected: z.boolean().optional(),
  returnCodeInterpretation: z.string().optional(),
  persistedOutputPath: z.string().optional(),
  persistedOutputSize: z.number().optional(),
}).passthrough();

export type BashToolResult = z.infer<typeof BashToolResultSchema>;

// ToolUseResult contains tool-specific metadata about what the tool returned.
// This is highly variable depending on the tool (Bash, Read, Grep, etc.). The
// typed Bash variant comes FIRST so Bash results parse into a known shape; the
// open z.record branch stays LAST as the forward-compatible catch-all.
const ToolUseResultSchema = z.union([
  z.string(), // Error messages or simple results
  BashToolResultSchema, // Typed Bash tool result (CC >= 2.1.143)
  z.array(z.unknown()), // Array of content blocks (e.g., MCP tool results)
  z.record(z.string(), z.unknown()), // Tool-specific structured data (catch-all)
]);

// Todo item from TodoWrite tool
const TodoItemSchema = z.object({
  content: z.string(),
  status: z.string(), // 'pending' | 'in_progress' | 'completed'
  activeForm: z.string(),
});

// Inline per-row permission mode (Claude Code >= 2.1.143). Five known values:
// 'default' | 'acceptEdits' | 'bypassPermissions' | 'plan' | 'auto' ('auto' is
// the risk-evaluating mode added in 2.1.143). Modeled as an optional string —
// NOT z.enum — so a future upstream mode keeps validating (read-both-forms
// convention). Documented in schemas/README.md.
const PermissionModeSchema = z.string().optional();

const UserMessageSchema = BaseMessageSchema.extend({
  type: z.literal('user'),
  thinkingMetadata: ThinkingMetadataSchema.optional(),
  slug: z.string().optional(), // Session slug for display
  todos: z.array(TodoItemSchema).nullable().optional(), // Todo list state
  isMeta: z.boolean().optional(), // Skill expansion messages have isMeta: true
  permissionMode: PermissionModeSchema,
  sourceToolUseID: z.string().optional(), // Links skill expansion to the Skill tool_use
  message: z.object({
    role: z.literal('user'),
    content: z.union([z.string(), z.array(ContentBlockSchema)]),
  }),
  toolUseResult: ToolUseResultSchema.optional(),
});

const AssistantMessageSchema = BaseMessageSchema.extend({
  type: z.literal('assistant'),
  requestId: z.string().optional(), // Optional for synthetic error messages
  agentId: z.string().optional(),
  permissionMode: PermissionModeSchema,
  message: z.object({
    model: z.string(),
    id: z.string(),
    type: z.literal('message'),
    role: z.literal('assistant'),
    content: z.array(ContentBlockSchema),
    stop_reason: z.string().nullable(), // 'end_turn' | 'tool_use' | 'max_tokens' | 'stop_sequence' | null
    stop_sequence: z.string().nullable(),
    usage: TokenUsageSchema,
  }),
  // CF-418: parse-layer-stamped canonical token usage. Not present on the wire.
  tokenUsage: NormalizedTokenUsageSchema.optional(),
});

const FileBackupSchema = z.object({
  backupFileName: z.string().nullable(),
  version: z.number(),
  backupTime: z.string(),
});

const FileHistorySnapshotSchema = z.object({
  type: z.literal('file-history-snapshot'),
  messageId: z.string(),
  isSnapshotUpdate: z.boolean(),
  snapshot: z.object({
    messageId: z.string(),
    timestamp: z.string(),
    trackedFileBackups: z.record(z.string(), FileBackupSchema),
  }),
});

const SystemMessageSchema = BaseMessageSchema.extend({
  type: z.literal('system'),
  logicalParentUuid: z.string().optional(),
  subtype: z.string(),
  content: z.string().optional(), // Not present for some subtypes (e.g., turn_duration)
  isMeta: z.boolean().optional(), // Optional - not present for api_error subtype
  level: z.string().optional(), // 'info' | 'warning' | 'error' - not present for some subtypes
  // Fields for turn_duration subtype
  durationMs: z.number().optional(),
  slug: z.string().optional(),
  compactMetadata: z
    .object({
      trigger: z.string(), // 'auto' | 'manual' - use string for forward compat
      preTokens: z.number(),
    })
    .optional(),
  // api_error subtype has additional fields: cause, error, retryInMs, retryAttempt, maxRetries
  // Use passthrough to preserve them
}).passthrough();

const SummaryMessageSchema = z.object({
  type: z.literal('summary'),
  summary: z.string(),
  leafUuid: z.string(),
});

const QueueOperationMessageSchema = z.object({
  type: z.literal('queue-operation'),
  operation: z.string(),
  timestamp: z.string(),
  content: z.string().optional(),
  sessionId: z.string(),
});

const PRLinkMessageSchema = z.object({
  type: z.literal('pr-link'),
  prNumber: z.number(),
  prRepository: z.string(),
  prUrl: z.string(),
  sessionId: z.string(),
  timestamp: z.string(),
});

// ============================================================================
// Attachment Subtype Schemas
// ============================================================================
//
// `attachment` rows are non-conversational JSONL records that Claude Code emits
// to capture side-channel events: hook output, out-of-band file edits, queued
// prompts, and mid-session tool-availability changes. Each row has an outer
// envelope (type: "attachment") and an inner `attachment` object whose own
// `type` field discriminates the subtype.
//
// We render 6 high-signal subtypes (grouped into 5 sub-chips in the UI) and
// preserve 3 noisy ones (`task_reminder`, `skill_listing`, `command_permissions`)
// + any future subtypes via a catch-all branch — those parse without error but
// no filter chip routes to them, so they don't render. This mirrors the
// forward-compat approach used by ContentBlockSchema and TranscriptLineSchema.

const HookSuccessAttachmentSchema = z.object({
  type: z.literal('hook_success'),
  hookName: z.string().optional(),
  hookEvent: z.string().optional(),
  toolUseID: z.string().optional(),
  command: z.string().optional(),
  content: z.string().optional(),
  stdout: z.string().optional(),
  stderr: z.string().optional(),
  exitCode: z.number().optional(),
  durationMs: z.number().optional(),
}).passthrough();

const HookBlockingErrorAttachmentSchema = z.object({
  type: z.literal('hook_blocking_error'),
  hookName: z.string().optional(),
  hookEvent: z.string().optional(),
  toolUseID: z.string().optional(),
  blockingError: z.object({
    blockingError: z.string(),
    command: z.string().optional(),
  }).passthrough(),
}).passthrough();

const EditedTextFileAttachmentSchema = z.object({
  type: z.literal('edited_text_file'),
  filename: z.string(),
  snippet: z.string(),
}).passthrough();

const QueuedCommandAttachmentSchema = z.object({
  type: z.literal('queued_command'),
  prompt: z.string(),
  commandMode: z.string().optional(),
}).passthrough();

const DeferredToolsDeltaAttachmentSchema = z.object({
  type: z.literal('deferred_tools_delta'),
  addedNames: z.array(z.string()).optional(),
  removedNames: z.array(z.string()).optional(),
  addedLines: z.array(z.string()).optional(),
}).passthrough();

const McpInstructionsDeltaAttachmentSchema = z.object({
  type: z.literal('mcp_instructions_delta'),
  addedNames: z.array(z.string()).optional(),
  removedNames: z.array(z.string()).optional(),
  addedBlocks: z.array(z.string()).optional(),
}).passthrough();

// Catch-all for noisy + unknown attachment subtypes — must be last in the union.
const UnknownAttachmentSchema = z.object({ type: z.string() }).passthrough();

const AttachmentInnerSchema = z.union([
  HookSuccessAttachmentSchema,
  HookBlockingErrorAttachmentSchema,
  EditedTextFileAttachmentSchema,
  QueuedCommandAttachmentSchema,
  DeferredToolsDeltaAttachmentSchema,
  McpInstructionsDeltaAttachmentSchema,
  UnknownAttachmentSchema,
]);

const AttachmentMessageSchema = z.object({
  type: z.literal('attachment'),
  uuid: z.string(),
  timestamp: z.string(),
  parentUuid: z.string().nullable().optional(),
  isSidechain: z.boolean().optional(),
  userType: z.string().optional(),
  entrypoint: z.string().optional(),
  cwd: z.string().optional(),
  sessionId: z.string().optional(),
  version: z.string().optional(),
  gitBranch: z.string().optional(),
  attachment: AttachmentInnerSchema,
}).passthrough();

// Catch-all for forward compatibility — must be last in the union.
// Unknown message types pass validation and render with a fallback UI.
const UnknownMessageSchema = z.object({ type: z.string() }).passthrough();

const TranscriptLineSchema = z.union([
  UserMessageSchema,
  AssistantMessageSchema,
  FileHistorySnapshotSchema,
  SystemMessageSchema,
  SummaryMessageSchema,
  QueueOperationMessageSchema,
  PRLinkMessageSchema,
  AttachmentMessageSchema,
  UnknownMessageSchema,
]);

export type TranscriptLine = z.infer<typeof TranscriptLineSchema>;

// ============================================================================
// Validation Functions
// ============================================================================

/**
 * Structured validation error for display in UI
 */
export interface TranscriptValidationError {
  line: number;
  rawJson: string;
  messageType?: string;
  errors: Array<{
    path: string;
    message: string;
    expected?: string;
    received?: string;
  }>;
}

/**
 * Result of parsing a transcript with detailed error information
 */
export interface TranscriptParseResult {
  messages: TranscriptLine[];
  errors: TranscriptValidationError[];
  totalLines: number;
  successCount: number;
  errorCount: number;
}

/**
 * Extract detailed errors from Zod issues, handling union errors specially.
 * For union errors, we find the branch that matched the type and show its errors.
 */
function extractDetailedErrors(
  issues: z.ZodIssue[],
  messageType: string | undefined
): Array<{ path: string; message: string; expected?: string; received?: string }> {
  const errors: Array<{ path: string; message: string; expected?: string; received?: string }> = [];

  for (const issue of issues) {
    if (issue.code === 'invalid_union') {
      // In Zod v4, invalid_union has `errors: ZodIssue[][]` - array of issue arrays for each branch
      const branchErrors = issue.errors;
      if (branchErrors && branchErrors.length > 0) {
        // Find the best branch to show errors from
        // Priority 1: Branch where the type matches (no type error means the type field was correct)
        // Priority 2: Branch with fewest total errors
        let bestBranchIssues: z.ZodIssue[] | undefined;
        let minIssueCount = Infinity;

        for (const branchIssueArray of branchErrors) {
          // Check if this branch has a type mismatch error
          const typeIssue = branchIssueArray.find(
            (i: z.ZodIssue) => i.path.length === 1 && i.path[0] === 'type' && i.code === 'invalid_value'
          );

          if (!typeIssue) {
            // No type error means this branch's type matched - this is our best match
            bestBranchIssues = branchIssueArray;
            break;
          }

          // Check if this branch expects our message type (even if it failed)
          if (typeIssue.code === 'invalid_value') {
            const expectedValues = typeIssue.values;
            if (messageType && expectedValues.includes(messageType)) {
              // This branch expects our type but has other errors
              bestBranchIssues = branchIssueArray;
              break;
            }
          }

          // Track the branch with fewest errors as fallback
          if (branchIssueArray.length < minIssueCount) {
            minIssueCount = branchIssueArray.length;
            bestBranchIssues = branchIssueArray;
          }
        }

        if (bestBranchIssues) {
          // Recursively extract errors from the best branch, excluding type mismatch errors
          const filteredIssues = bestBranchIssues.filter(
            (i: z.ZodIssue) => !(i.path.length === 1 && i.path[0] === 'type' && i.code === 'invalid_value')
          );
          errors.push(...extractDetailedErrors(filteredIssues, messageType));
        }
      }
    } else {
      // Regular error - format it directly
      errors.push({
        path: issue.path.length > 0 ? issue.path.join('.') : '(root)',
        message: issue.message,
        expected: 'expected' in issue ? String(issue.expected) : undefined,
        received: 'received' in issue ? String(issue.received) : undefined,
      });
    }
  }

  return errors;
}

/**
 * Format a Zod error into a human-readable structure.
 * Accepts the parsed object directly to avoid redundant JSON.parse calls.
 */
function formatZodError(
  error: z.ZodError,
  rawJson: string,
  lineIndex: number,
  parsed?: unknown
): TranscriptValidationError {
  // Extract message type from parsed object (or fall back to parsing raw JSON)
  let messageType: string | undefined;
  const obj = parsed ?? safeJsonParse(rawJson);
  if (obj !== null && typeof obj === 'object' && 'type' in obj) {
    messageType = String(obj.type);
  }

  // Extract detailed errors, handling union types specially
  const detailedErrors = extractDetailedErrors(error.issues, messageType);

  // If we couldn't extract detailed errors, fall back to showing the raw Zod error
  const finalErrors = detailedErrors.length > 0 ? detailedErrors : error.issues.map((issue) => ({
    path: issue.path.length > 0 ? issue.path.join('.') : '(root)',
    message: issue.message,
    expected: 'expected' in issue ? String(issue.expected) : undefined,
    received: 'received' in issue ? String(issue.received) : undefined,
  }));

  return {
    line: lineIndex + 1,
    rawJson, // Keep full raw JSON for debugging
    messageType,
    errors: finalErrors,
  };
}

/** Safe JSON.parse that returns null on failure */
function safeJsonParse(json: string): unknown {
  try {
    return JSON.parse(json);
  } catch {
    return null;
  }
}

/**
 * Format validation errors for console logging
 */
export function formatValidationErrorsForLog(errors: TranscriptValidationError[]): string {
  if (errors.length === 0) return '';

  const lines = [`Transcript validation errors (${errors.length} total):`];

  for (const err of errors.slice(0, 10)) {
    lines.push(`  Line ${err.line}${err.messageType ? ` (type: ${err.messageType})` : ''}:`);
    for (const e of err.errors) {
      let detail = `    - ${e.path}: ${e.message}`;
      if (e.expected && e.received) {
        detail += ` (expected: ${e.expected}, got: ${e.received})`;
      }
      lines.push(detail);
    }
    // Log truncated raw JSON for debugging
    const truncatedRaw = err.rawJson.length > 500 ? err.rawJson.slice(0, 500) + '...' : err.rawJson;
    lines.push(`    Raw: ${truncatedRaw}`);
  }

  if (errors.length > 10) {
    lines.push(`  ... and ${errors.length - 10} more errors`);
  }

  return lines.join('\n');
}

type TranscriptLineResult =
  | { success: true; data: TranscriptLine }
  | { success: false; error: TranscriptValidationError };

/**
 * Validate a pre-parsed object against the TranscriptLine schema.
 * Use this when JSON has already been parsed to avoid double-parsing.
 */
export function validateParsedTranscriptLine(
  parsed: unknown,
  rawLine: string,
  lineIndex: number
): TranscriptLineResult {
  const result = TranscriptLineSchema.safeParse(parsed);
  if (result.success) {
    return { success: true, data: result.data };
  }

  return {
    success: false,
    error: formatZodError(result.error, rawLine, lineIndex, parsed),
  };
}

/**
 * Parse transcript line from raw JSON string with structured error for UI display
 */
export function parseTranscriptLineWithError(
  rawLine: string,
  lineIndex: number
): TranscriptLineResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(rawLine);
  } catch (e) {
    return {
      success: false,
      error: {
        line: lineIndex + 1,
        rawJson: rawLine.length > 200 ? rawLine.slice(0, 200) + '...' : rawLine,
        errors: [{
          path: '(root)',
          message: `Invalid JSON: ${e instanceof Error ? e.message : 'parse error'}`,
        }],
      },
    };
  }

  return validateParsedTranscriptLine(parsed, rawLine, lineIndex);
}

// ============================================================================
// Forward-compatibility: known type lists for schema drift detection
// ============================================================================

const KNOWN_BLOCK_TYPES = ['text', 'thinking', 'tool_use', 'tool_result', 'image', 'tool_reference'];
const KNOWN_MESSAGE_TYPES = ['user', 'assistant', 'system', 'file-history-snapshot', 'summary', 'queue-operation', 'pr-link', 'attachment'];
const _warnedTypes = new Set<string>();

/**
 * Warn (once per type) when the catch-all schema matches a type string
 * that has a dedicated schema. This indicates the specific schema has drifted
 * (e.g., a new required field was added upstream).
 */
export function warnIfKnownTypeCaughtByCatchall(kind: 'block' | 'message', type: string): void {
  const knownTypes = kind === 'block' ? KNOWN_BLOCK_TYPES : KNOWN_MESSAGE_TYPES;
  const key = `${kind}:${type}`;
  if (knownTypes.includes(type) && !_warnedTypes.has(key)) {
    _warnedTypes.add(key);
    console.warn(
      `${kind === 'block' ? 'Content block' : 'Message'} type "${type}" matched catch-all schema ` +
      `(expected fields may be missing). This suggests schema drift — update the specific schema.`
    );
  }
}

// ============================================================================
// Type Guards (derived from schemas)
// ============================================================================

export function isTextBlock(block: ContentBlock): block is TextBlock {
  return block.type === 'text';
}

export function isThinkingBlock(block: ContentBlock): block is ThinkingBlock {
  return block.type === 'thinking';
}

export function isToolUseBlock(block: ContentBlock): block is ToolUseBlock {
  return block.type === 'tool_use';
}

export function isToolResultBlock(
  block: ContentBlock
): block is { type: 'tool_result'; tool_use_id: string; content: string | ContentBlock[]; is_error?: boolean } {
  return block.type === 'tool_result';
}

export function isImageBlock(block: ContentBlock): block is ImageBlock {
  return block.type === 'image';
}

export function isToolReferenceBlock(block: ContentBlock): block is ToolReferenceBlock {
  return block.type === 'tool_reference';
}

export function isUserMessage(line: TranscriptLine): line is z.infer<typeof UserMessageSchema> {
  return line.type === 'user';
}

export function isAssistantMessage(line: TranscriptLine): line is z.infer<typeof AssistantMessageSchema> {
  return line.type === 'assistant';
}

export function isSystemMessage(line: TranscriptLine): line is z.infer<typeof SystemMessageSchema> {
  return line.type === 'system';
}

export function isFileHistorySnapshot(line: TranscriptLine): line is z.infer<typeof FileHistorySnapshotSchema> {
  return line.type === 'file-history-snapshot';
}

export function isSummaryMessage(line: TranscriptLine): line is z.infer<typeof SummaryMessageSchema> {
  return line.type === 'summary';
}

export function isQueueOperationMessage(line: TranscriptLine): line is z.infer<typeof QueueOperationMessageSchema> {
  return line.type === 'queue-operation';
}

export function isPRLinkMessage(line: TranscriptLine): line is z.infer<typeof PRLinkMessageSchema> {
  return line.type === 'pr-link';
}

export function isAttachmentMessage(line: TranscriptLine): line is AttachmentMessage {
  return line.type === 'attachment'
    && 'attachment' in line
    && line.attachment !== null
    && typeof line.attachment === 'object';
}

// Type-predicate discriminators on the inner attachment. Each one narrows
// `msg.attachment` to the matching branch type, so call sites can read the
// subtype-specific fields without further assertions.
export function isHookSuccessAttachment(
  msg: AttachmentMessage,
): msg is AttachmentMessage & { attachment: HookSuccessAttachment } {
  return msg.attachment.type === 'hook_success';
}

export function isHookBlockingErrorAttachment(
  msg: AttachmentMessage,
): msg is AttachmentMessage & { attachment: HookBlockingErrorAttachment } {
  return msg.attachment.type === 'hook_blocking_error';
}

export function isEditedTextFileAttachment(
  msg: AttachmentMessage,
): msg is AttachmentMessage & { attachment: EditedTextFileAttachment } {
  return msg.attachment.type === 'edited_text_file';
}

export function isQueuedCommandAttachment(
  msg: AttachmentMessage,
): msg is AttachmentMessage & { attachment: QueuedCommandAttachment } {
  return msg.attachment.type === 'queued_command';
}

export function isDeferredToolsDeltaAttachment(
  msg: AttachmentMessage,
): msg is AttachmentMessage & { attachment: DeferredToolsDeltaAttachment } {
  return msg.attachment.type === 'deferred_tools_delta';
}

export function isMcpInstructionsDeltaAttachment(
  msg: AttachmentMessage,
): msg is AttachmentMessage & { attachment: McpInstructionsDeltaAttachment } {
  return msg.attachment.type === 'mcp_instructions_delta';
}

export function isUnknownBlock(block: ContentBlock): block is UnknownBlock {
  return !KNOWN_BLOCK_TYPES.includes(block.type);
}

export type UnknownMessage = { type: string; [key: string]: unknown };

export function isUnknownMessage(line: TranscriptLine): line is UnknownMessage {
  return !KNOWN_MESSAGE_TYPES.includes(line.type);
}

// ============================================================================
// Inferred Message Types
// ============================================================================

export type UserMessage = z.infer<typeof UserMessageSchema>;
export type AssistantMessage = z.infer<typeof AssistantMessageSchema>;
export type SystemMessage = z.infer<typeof SystemMessageSchema>;
export type PRLinkMessage = z.infer<typeof PRLinkMessageSchema>;
export type AttachmentMessage = z.infer<typeof AttachmentMessageSchema>;
export type HookSuccessAttachment = z.infer<typeof HookSuccessAttachmentSchema>;
export type HookBlockingErrorAttachment = z.infer<typeof HookBlockingErrorAttachmentSchema>;
export type EditedTextFileAttachment = z.infer<typeof EditedTextFileAttachmentSchema>;
export type QueuedCommandAttachment = z.infer<typeof QueuedCommandAttachmentSchema>;
export type DeferredToolsDeltaAttachment = z.infer<typeof DeferredToolsDeltaAttachmentSchema>;
export type McpInstructionsDeltaAttachment = z.infer<typeof McpInstructionsDeltaAttachmentSchema>;

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Check if assistant message contains thinking
 */
export function hasThinking(message: AssistantMessage): boolean {
  return message.message.content.some(isThinkingBlock);
}

/**
 * Check if assistant message uses tools
 */
export function usesTools(message: AssistantMessage): boolean {
  return message.message.content.some(isToolUseBlock);
}

/**
 * Check if user message is a tool result
 */
export function isToolResultMessage(message: UserMessage): boolean {
  const content = message.message.content;
  if (typeof content === 'string') return false;
  return content.some(isToolResultBlock);
}

/**
 * Check if user message is a skill expansion (injected skill content)
 */
export function isSkillExpansionMessage(message: UserMessage): boolean {
  return message.isMeta === true && message.sourceToolUseID !== undefined;
}

/**
 * Check if user message is a command-expansion skill invocation.
 * These have string content containing <command-name>/skillname</command-name>.
 */
export function isCommandExpansionMessage(message: UserMessage): boolean {
  const content = message.message.content;
  return typeof content === 'string' && content.includes('<command-name>');
}

/**
 * Extract skill name from a command-expansion message.
 * Returns null if not a command-expansion message.
 */
export function getCommandExpansionSkillName(message: UserMessage): string | null {
  const content = message.message.content;
  if (typeof content !== 'string') return null;
  const match = content.match(/<command-name>\/?(.+?)<\/command-name>/);
  return match?.[1] ?? null;
}

/**
 * Strip command-expansion XML tags from message content for clean display.
 * Removes <command-message>...</command-message> and <command-name>...</command-name> tags.
 */
export function stripCommandExpansionTags(content: string): string {
  return content
    .replace(/<command-message>[\s\S]*?<\/command-message>/g, '')
    .replace(/<command-name>[\s\S]*?<\/command-name>/g, '')
    .trim();
}

