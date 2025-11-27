// Zod schemas for Claude Code transcript validation
// Validates external transcript data at parse time to ensure type safety
//
// IMPORTANT: These schemas validate external data we don't control.
// Use string() instead of enum() for fields that could have new values added.
// Use passthrough() to preserve unknown fields for forward compatibility.
import { z } from 'zod';

// ============================================================================
// Content Block Schemas
// ============================================================================

export const TextBlockSchema = z.object({
  type: z.literal('text'),
  text: z.string(),
});

export const ThinkingBlockSchema = z.object({
  type: z.literal('thinking'),
  thinking: z.string(),
  signature: z.string().optional(), // Optional for forward compat
});

export const ToolUseBlockSchema = z.object({
  type: z.literal('tool_use'),
  id: z.string(),
  name: z.string(),
  input: z.record(z.string(), z.unknown()),
});

// ToolResultBlock can contain nested content blocks or a string
export const ToolResultBlockSchema = z.object({
  type: z.literal('tool_result'),
  tool_use_id: z.string(),
  content: z.union([z.string(), z.array(z.lazy(() => ContentBlockSchema))]),
  is_error: z.boolean().optional(),
});

export const ImageBlockSchema = z.object({
  type: z.literal('image'),
  source: z.object({
    type: z.string(), // 'base64' | 'url' - use string for forward compat
    media_type: z.string(),
    data: z.string().optional(),
    url: z.string().optional(),
  }),
});

export const ContentBlockSchema: z.ZodType<ContentBlock> = z.union([
  TextBlockSchema,
  ThinkingBlockSchema,
  ToolUseBlockSchema,
  ToolResultBlockSchema,
  ImageBlockSchema,
]);

// Infer types from schemas
export type TextBlock = z.infer<typeof TextBlockSchema>;
export type ThinkingBlock = z.infer<typeof ThinkingBlockSchema>;
export type ToolUseBlock = z.infer<typeof ToolUseBlockSchema>;
export type ImageBlock = z.infer<typeof ImageBlockSchema>;

// ContentBlock needs manual definition due to recursion
export type ContentBlock =
  | TextBlock
  | ThinkingBlock
  | ToolUseBlock
  | { type: 'tool_result'; tool_use_id: string; content: string | ContentBlock[]; is_error?: boolean }
  | ImageBlock;

// ============================================================================
// Token Usage Schema
// ============================================================================

export const TokenUsageSchema = z.object({
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

export const ThinkingMetadataSchema = z.object({
  level: z.string(), // 'high' | 'medium' | 'low' | 'off' - use string for forward compat
  disabled: z.boolean(),
  triggers: z.array(z.string()),
});

// ToolUseResult contains tool-specific metadata about what the tool returned.
// This is highly variable depending on the tool (Bash, Read, Grep, etc.) so we use
// a flexible schema that accepts any structure.
export const ToolUseResultSchema = z.union([
  z.string(), // Error messages or simple results
  z.record(z.string(), z.unknown()), // Tool-specific structured data
]);

// Todo item from TodoWrite tool
export const TodoItemSchema = z.object({
  content: z.string(),
  status: z.string(), // 'pending' | 'in_progress' | 'completed'
  activeForm: z.string(),
});

export const UserMessageSchema = BaseMessageSchema.extend({
  type: z.literal('user'),
  thinkingMetadata: ThinkingMetadataSchema.optional(),
  slug: z.string().optional(), // Session slug for display
  todos: z.array(TodoItemSchema).nullable().optional(), // Todo list state
  message: z.object({
    role: z.literal('user'),
    content: z.union([z.string(), z.array(ContentBlockSchema)]),
  }),
  toolUseResult: ToolUseResultSchema.optional(),
});

export const AssistantMessageSchema = BaseMessageSchema.extend({
  type: z.literal('assistant'),
  requestId: z.string().optional(), // Optional for synthetic error messages
  agentId: z.string().optional(),
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
});

export const FileBackupSchema = z.object({
  backupFileName: z.string().nullable(),
  version: z.number(),
  backupTime: z.string(),
});

export const FileHistorySnapshotSchema = z.object({
  type: z.literal('file-history-snapshot'),
  messageId: z.string(),
  isSnapshotUpdate: z.boolean(),
  snapshot: z.object({
    messageId: z.string(),
    timestamp: z.string(),
    trackedFileBackups: z.record(z.string(), FileBackupSchema),
  }),
});

export const SystemMessageSchema = BaseMessageSchema.extend({
  type: z.literal('system'),
  logicalParentUuid: z.string().optional(),
  subtype: z.string(),
  content: z.string(),
  isMeta: z.boolean(),
  level: z.string(), // 'info' | 'warning' | 'error' - use string for forward compat
  compactMetadata: z
    .object({
      trigger: z.string(), // 'auto' | 'manual' - use string for forward compat
      preTokens: z.number(),
    })
    .optional(),
});

export const SummaryMessageSchema = z.object({
  type: z.literal('summary'),
  summary: z.string(),
  leafUuid: z.string(),
});

export const QueueOperationMessageSchema = z.object({
  type: z.literal('queue-operation'),
  operation: z.string(),
  timestamp: z.string(),
  content: z.string(),
  sessionId: z.string(),
});

export const TranscriptLineSchema = z.union([
  UserMessageSchema,
  AssistantMessageSchema,
  FileHistorySnapshotSchema,
  SystemMessageSchema,
  SummaryMessageSchema,
  QueueOperationMessageSchema,
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
 * Format a Zod error into a human-readable structure
 */
function formatZodError(error: z.ZodError, rawJson: string, lineIndex: number): TranscriptValidationError {
  // Try to extract type from raw JSON for context
  let messageType: string | undefined;
  try {
    const parsed = JSON.parse(rawJson);
    if (typeof parsed === 'object' && parsed !== null && 'type' in parsed) {
      messageType = String(parsed.type);
    }
  } catch {
    // Ignore - raw JSON may not be valid
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
  }

  if (errors.length > 10) {
    lines.push(`  ... and ${errors.length - 10} more errors`);
  }

  return lines.join('\n');
}

/**
 * Parse and validate a single transcript line.
 * Returns the validated data or throws a detailed error.
 */
export function parseTranscriptLine(data: unknown): TranscriptLine {
  return TranscriptLineSchema.parse(data);
}

/**
 * Safely parse a transcript line, returning null on failure.
 * Logs validation errors for debugging.
 */
export function safeParseTranscriptLine(
  data: unknown,
  lineIndex?: number
): { success: true; data: TranscriptLine } | { success: false; error: z.ZodError } {
  const result = TranscriptLineSchema.safeParse(data);
  if (!result.success) {
    const prefix = lineIndex !== undefined ? `Line ${lineIndex + 1}: ` : '';
    console.warn(`${prefix}Transcript validation failed:`, result.error.issues);
  }
  return result;
}

/**
 * Parse transcript line with structured error for UI display
 */
export function parseTranscriptLineWithError(
  rawLine: string,
  lineIndex: number
): { success: true; data: TranscriptLine } | { success: false; error: TranscriptValidationError } {
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

  const result = TranscriptLineSchema.safeParse(parsed);
  if (result.success) {
    return { success: true, data: result.data };
  }

  return {
    success: false,
    error: formatZodError(result.error, rawLine, lineIndex),
  };
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

// ============================================================================
// Inferred Message Types
// ============================================================================

export type UserMessage = z.infer<typeof UserMessageSchema>;
export type AssistantMessage = z.infer<typeof AssistantMessageSchema>;
export type SystemMessage = z.infer<typeof SystemMessageSchema>;
export type FileHistorySnapshot = z.infer<typeof FileHistorySnapshotSchema>;
export type SummaryMessage = z.infer<typeof SummaryMessageSchema>;
export type QueueOperationMessage = z.infer<typeof QueueOperationMessageSchema>;
export type ToolResultBlock = { type: 'tool_result'; tool_use_id: string; content: string | ContentBlock[]; is_error?: boolean };

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
 * Get all tool uses from an assistant message
 */
export function getToolUses(message: AssistantMessage): ToolUseBlock[] {
  return message.message.content.filter(isToolUseBlock);
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
 * Get tool results from user message
 */
export function getToolResults(message: UserMessage): ToolResultBlock[] {
  const content = message.message.content;
  if (typeof content === 'string') return [];
  return content.filter(isToolResultBlock);
}

/**
 * Get message content as plain text (strips formatting)
 */
export function getPlainTextContent(content: ContentBlock[]): string {
  return content
    .filter(isTextBlock)
    .map((block) => block.text)
    .join('\n');
}
