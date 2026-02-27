import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';
import { isToolResultMessage, isToolResultBlock, isSkillExpansionMessage, isUserMessage, isAssistantMessage, isTextBlock, isThinkingBlock, isToolUseBlock } from '@/types';

// Message categories for filtering - matches top-level transcript types
export type MessageCategory = 'user' | 'assistant' | 'system' | 'file-history-snapshot' | 'summary' | 'queue-operation' | 'pr-link' | 'unknown';

// Subcategory types for hierarchical filtering
export type UserSubcategory = 'prompt' | 'tool-result' | 'skill';
export type AssistantSubcategory = 'text' | 'tool-use' | 'thinking';

// Subcategory counts for hierarchical categories
export interface UserSubcategoryCounts {
  prompt: number;
  'tool-result': number;
  skill: number;
}

export interface AssistantSubcategoryCounts {
  text: number;
  'tool-use': number;
  thinking: number;
}

// Hierarchical counts structure
export interface HierarchicalCounts {
  user: { total: number } & UserSubcategoryCounts;
  assistant: { total: number } & AssistantSubcategoryCounts;
  system: number;
  'file-history-snapshot': number;
  summary: number;
  'queue-operation': number;
  'pr-link': number;
  unknown: number;
}


// Filter state - tracks which subcategories are visible
export interface FilterState {
  user: { prompt: boolean; 'tool-result': boolean; skill: boolean };
  assistant: { text: boolean; 'tool-use': boolean; thinking: boolean };
  system: boolean;
  'file-history-snapshot': boolean;
  summary: boolean;
  'queue-operation': boolean;
  'pr-link': boolean;
  unknown: boolean;
}

// Default filter state (user and assistant visible with all subs, others hidden)
export const DEFAULT_FILTER_STATE: FilterState = {
  user: { prompt: true, 'tool-result': true, skill: true },
  assistant: { text: true, 'tool-use': true, thinking: true },
  system: false,
  'file-history-snapshot': false,
  summary: false,
  'queue-operation': false,
  'pr-link': false,
  unknown: true,
};

/**
 * Get the subcategory for a user message
 * Priority: skill > tool-result > prompt
 */
function categorizeUserMessage(message: UserMessage): UserSubcategory {
  if (isSkillExpansionMessage(message)) return 'skill';
  if (isToolResultMessage(message)) return 'tool-result';
  return 'prompt';
}

/**
 * Get the subcategory for an assistant message.
 * Priority: thinking > tool-use > text (a message can have multiple block types)
 */
function categorizeAssistantMessage(message: AssistantMessage): AssistantSubcategory {
  const content = message.message.content;

  if (content.some(isThinkingBlock)) return 'thinking';
  if (content.some(isToolUseBlock)) return 'tool-use';
  return 'text';
}

/**
 * Count messages in each category with hierarchical subcategories
 */
export function countHierarchicalCategories(messages: TranscriptLine[]): HierarchicalCounts {
  const counts: HierarchicalCounts = {
    user: { total: 0, prompt: 0, 'tool-result': 0, skill: 0 },
    assistant: { total: 0, text: 0, 'tool-use': 0, thinking: 0 },
    system: 0,
    'file-history-snapshot': 0,
    summary: 0,
    'queue-operation': 0,
    'pr-link': 0,
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
export function getRoleLabel(message: TranscriptLine): string {
  if (isUserMessage(message)) {
    const content = message.message.content;
    if (Array.isArray(content)) {
      if (content.some(isToolResultBlock)) return 'Tool';
    }
    return 'User';
  }
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
 * Check if a message has any displayable content.
 * Messages with only whitespace text (e.g. interrupted assistant responses
 * with content: [{type: "text", text: "\n\n"}]) render as empty cards.
 */
function hasDisplayableContent(message: TranscriptLine): boolean {
  if (isAssistantMessage(message)) {
    const content = message.message.content;
    if (content.length === 0) return false;
    // Has displayable content if any block is non-text (tool_use, thinking)
    // or is a text block with non-whitespace content
    return content.some((block) => {
      if (isTextBlock(block)) return block.text.trim().length > 0;
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
export function messageMatchesFilter(message: TranscriptLine, filterState: FilterState): boolean {
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
