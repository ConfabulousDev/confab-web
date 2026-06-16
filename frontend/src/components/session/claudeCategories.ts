import type { TranscriptLine, UserMessage, AssistantMessage, AttachmentMessage, SystemMessage } from '@/types';
import {
  isToolResultMessage,
  isToolResultBlock,
  isSkillExpansionMessage,
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isAttachmentMessage,
  isHookSuccessAttachment,
  isHookBlockingErrorAttachment,
  isEditedTextFileAttachment,
  isQueuedCommandAttachment,
  isDeferredToolsDeltaAttachment,
  isMcpInstructionsDeltaAttachment,
  isTextBlock,
  isThinkingBlock,
  isToolUseBlock,
} from '@/types';

// Message categories for filtering - matches top-level transcript types plus
// the synthetic categories introduced in CF-346.
export type ClaudeCategory =
  | 'user'
  | 'assistant'
  | 'system'
  | 'file-history-snapshot'
  | 'summary'
  | 'queue-operation'
  | 'pr-link'
  | 'attachment'
  | 'away-summary'
  | 'unknown';

// Subcategory types for hierarchical filtering
export type ClaudeUserSubcategory = 'prompt' | 'tool-result' | 'skill';
export type ClaudeAssistantSubcategory = 'text' | 'tool-use' | 'thinking';
export type ClaudeAttachmentSubcategory =
  | 'hook'
  | 'file-edit'
  | 'queued-command'
  | 'deferred-tools'
  | 'mcp-instructions';

// Subcategory counts for hierarchical categories
export interface ClaudeUserSubcategoryCounts {
  prompt: number;
  'tool-result': number;
  skill: number;
}

export interface ClaudeAssistantSubcategoryCounts {
  text: number;
  'tool-use': number;
  thinking: number;
}

export interface ClaudeAttachmentSubcategoryCounts {
  hook: number;
  'file-edit': number;
  'queued-command': number;
  'deferred-tools': number;
  'mcp-instructions': number;
}

// Hierarchical counts structure
export interface ClaudeHierarchicalCounts {
  user: { total: number } & ClaudeUserSubcategoryCounts;
  assistant: { total: number } & ClaudeAssistantSubcategoryCounts;
  attachment: { total: number } & ClaudeAttachmentSubcategoryCounts;
  system: number;
  'file-history-snapshot': number;
  summary: number;
  'queue-operation': number;
  'pr-link': number;
  'away-summary': number;
  unknown: number;
}

// Filter state - tracks which subcategories are visible
export interface ClaudeFilterState {
  user: { prompt: boolean; 'tool-result': boolean; skill: boolean };
  assistant: { text: boolean; 'tool-use': boolean; thinking: boolean };
  attachment: {
    hook: boolean;
    'file-edit': boolean;
    'queued-command': boolean;
    'deferred-tools': boolean;
    'mcp-instructions': boolean;
  };
  system: boolean;
  'file-history-snapshot': boolean;
  summary: boolean;
  'queue-operation': boolean;
  'pr-link': boolean;
  'away-summary': boolean;
  unknown: boolean;
}

// Default filter state: user and assistant visible with all subs; attachments,
// away-summary, and the other side-channel categories all hidden (opt-in).
export const DEFAULT_CLAUDE_FILTER_STATE: ClaudeFilterState = {
  user: { prompt: true, 'tool-result': true, skill: true },
  assistant: { text: true, 'tool-use': true, thinking: true },
  attachment: {
    hook: false,
    'file-edit': false,
    'queued-command': false,
    'deferred-tools': false,
    'mcp-instructions': false,
  },
  system: false,
  'file-history-snapshot': false,
  summary: false,
  'queue-operation': false,
  'pr-link': false,
  'away-summary': false,
  unknown: true,
};

/**
 * Get the subcategory for a user message.
 * Priority: skill > tool-result > prompt
 */
function categorizeUserMessage(message: UserMessage): ClaudeUserSubcategory {
  if (isSkillExpansionMessage(message)) return 'skill';
  if (isToolResultMessage(message)) return 'tool-result';
  return 'prompt';
}

/**
 * Get the subcategory for an assistant message.
 * Priority: thinking > tool-use > text (a message can have multiple block types)
 */
function categorizeAssistantMessage(message: AssistantMessage): ClaudeAssistantSubcategory {
  const content = message.message.content;

  if (content.some(isThinkingBlock)) return 'thinking';
  if (content.some(isToolUseBlock)) return 'tool-use';
  return 'text';
}

/**
 * Get the sub-chip an attachment row belongs to, or null for noisy / unknown
 * subtypes that no filter chip routes to (task_reminder, skill_listing,
 * command_permissions, or any future subtype). Returning null means the row
 * is parsed but never rendered.
 */
function categorizeAttachmentMessage(message: AttachmentMessage): ClaudeAttachmentSubcategory | null {
  if (isHookSuccessAttachment(message) || isHookBlockingErrorAttachment(message)) return 'hook';
  if (isEditedTextFileAttachment(message)) return 'file-edit';
  if (isQueuedCommandAttachment(message)) return 'queued-command';
  if (isDeferredToolsDeltaAttachment(message)) return 'deferred-tools';
  if (isMcpInstructionsDeltaAttachment(message)) return 'mcp-instructions';
  return null;
}

/**
 * `system` rows where `subtype === 'away_summary'` are surfaced under their own
 * chip; this helper centralizes the test so counters and the filter agree.
 * Exported as a type guard so the timeline renderer dispatches off the same
 * predicate and gets the `SystemMessage` narrowing the AwaySummary view needs.
 */
export function isAwaySummaryMessage(message: TranscriptLine): message is SystemMessage {
  return isSystemMessage(message) && message.subtype === 'away_summary';
}

/**
 * `system` rows where `subtype === 'informational'` are Claude Code's
 * onboarding banners (CC >= 2.1.143), e.g. the "auto mode" notice. They render
 * as a styled callout (InformationalBanner) and carry their own role label, but
 * stay bucketed under the `system` filter chip — no separate chip at MVP.
 * Exported as a type guard so the timeline renderer dispatches off the same
 * predicate and gets the `SystemMessage` narrowing the InformationalBanner view
 * needs.
 */
export function isInformationalMessage(message: TranscriptLine): message is SystemMessage {
  return isSystemMessage(message) && message.subtype === 'informational';
}

/**
 * Count messages in each category with hierarchical subcategories
 */
export function countClaudeCategories(messages: TranscriptLine[]): ClaudeHierarchicalCounts {
  const counts: ClaudeHierarchicalCounts = {
    user: { total: 0, prompt: 0, 'tool-result': 0, skill: 0 },
    assistant: { total: 0, text: 0, 'tool-use': 0, thinking: 0 },
    attachment: {
      total: 0,
      hook: 0,
      'file-edit': 0,
      'queued-command': 0,
      'deferred-tools': 0,
      'mcp-instructions': 0,
    },
    system: 0,
    'file-history-snapshot': 0,
    summary: 0,
    'queue-operation': 0,
    'pr-link': 0,
    'away-summary': 0,
    unknown: 0,
  };

  for (const message of messages) {
    if (isUserMessage(message)) {
      counts.user.total++;
      const subcategory = categorizeUserMessage(message);
      counts.user[subcategory]++;
    } else if (isAssistantMessage(message)) {
      counts.assistant.total++;
      const subcategory = categorizeAssistantMessage(message);
      counts.assistant[subcategory]++;
    } else if (isAttachmentMessage(message)) {
      // Only rendered subs increment the parent total (per CF-346 decision #11).
      const subcategory = categorizeAttachmentMessage(message);
      if (subcategory !== null) {
        counts.attachment.total++;
        counts.attachment[subcategory]++;
      }
    } else if (isAwaySummaryMessage(message)) {
      // away_summary system rows are bucketed to their own chip, not `system`.
      counts['away-summary']++;
    } else {
      // Flat categories - increment the specific counter, unknown as fallback
      const msgType = message.type;
      if (msgType === 'system') {
        counts.system++;
      } else if (msgType === 'file-history-snapshot') {
        counts['file-history-snapshot']++;
      } else if (msgType === 'summary') {
        counts.summary++;
      } else if (msgType === 'queue-operation') {
        counts['queue-operation']++;
      } else if (msgType === 'pr-link') {
        counts['pr-link']++;
      } else {
        counts.unknown++;
      }
    }
  }

  return counts;
}

/**
 * Get role label for display (used by TimelineMessage header and skip navigation)
 */
export function getClaudeRoleLabel(message: TranscriptLine): string {
  if (isUserMessage(message)) {
    const content = message.message.content;
    if (Array.isArray(content)) {
      if (content.some(isToolResultBlock)) return 'Tool';
    }
    return 'User';
  }
  if (isAttachmentMessage(message)) return 'Attachment';
  if (isAwaySummaryMessage(message)) return 'Resume Summary';
  if (isInformationalMessage(message)) return 'Notice';
  switch (message.type) {
    case 'assistant':
      return 'Assistant';
    case 'system':
      return 'System';
    case 'summary':
      return 'Summary';
    case 'file-history-snapshot':
      return 'File Snapshot';
    case 'queue-operation':
      return 'Queue';
    default:
      return 'Unknown';
  }
}

/**
 * CF-574: true only when a message's type lands in the catch-all "unknown"
 * bucket — a genuine parser gap, NOT a known-but-unlabeled type such as
 * `pr-link` (which `getClaudeRoleLabel` also renders as "Unknown"). Mirrors the
 * flat-category fall-through in `claudeItemMatchesFilter` so the "Report this
 * message" affordance only appears on truly unrecognized rows.
 */
export function isUnknownClaudeMessage(message: TranscriptLine): boolean {
  if (isUserMessage(message) || isAssistantMessage(message)) return false;
  if (isAttachmentMessage(message) || isAwaySummaryMessage(message)) return false;
  switch (message.type) {
    case 'system':
    case 'summary':
    case 'file-history-snapshot':
    case 'queue-operation':
    case 'pr-link':
      return false;
    default:
      return true;
  }
}

/**
 * Check if a message has any displayable content.
 * Messages with only whitespace text (e.g. interrupted assistant responses
 * with content: [{type: "text", text: "\n\n"}]) render as empty cards.
 */
function hasDisplayableContent(message: TranscriptLine): boolean {
  if (isAssistantMessage(message)) {
    const content = message.message.content;
    if (content.length === 0) return false;
    // Has displayable content if any block has non-empty content
    return content.some((block) => {
      if (isTextBlock(block)) return block.text.trim().length > 0;
      if (isThinkingBlock(block)) return block.thinking.trim().length > 0;
      return true;
    });
  }
  if (isUserMessage(message)) {
    const content = message.message.content;
    if (typeof content === 'string') return content.trim().length > 0;
    if (content.length === 0) return false;
    return content.some((block) => {
      if (isTextBlock(block)) return block.text.trim().length > 0;
      return true;
    });
  }
  return true;
}

/**
 * Check if a message matches the current filter state
 */
export function claudeItemMatchesFilter(message: TranscriptLine, filterState: ClaudeFilterState): boolean {
  // Skip messages with no displayable content (e.g. interrupted empty responses)
  if (!hasDisplayableContent(message)) return false;

  if (isUserMessage(message)) {
    const subcategory = categorizeUserMessage(message);
    return filterState.user[subcategory];
  }

  if (isAssistantMessage(message)) {
    const subcategory = categorizeAssistantMessage(message);
    return filterState.assistant[subcategory];
  }

  if (isAttachmentMessage(message)) {
    const subcategory = categorizeAttachmentMessage(message);
    // Noisy/unknown subtypes are hidden regardless of chip state.
    if (subcategory === null) return false;
    return filterState.attachment[subcategory];
  }

  if (isAwaySummaryMessage(message)) {
    return filterState['away-summary'];
  }

  // Flat categories - check the specific filter state
  const msgType = message.type;
  if (msgType === 'system') return filterState.system;
  if (msgType === 'file-history-snapshot') return filterState['file-history-snapshot'];
  if (msgType === 'summary') return filterState.summary;
  if (msgType === 'queue-operation') return filterState['queue-operation'];
  if (msgType === 'pr-link') return filterState['pr-link'];

  // Unknown message types — controlled by the unknown filter chip
  return filterState.unknown;
}
