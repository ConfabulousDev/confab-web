import type { TranscriptLine } from '@/types';

// Message categories for filtering - matches top-level transcript types
export type MessageCategory = 'user' | 'assistant' | 'system' | 'file-history-snapshot' | 'summary' | 'queue-operation';

export interface MessageCategoryCounts {
  user: number;
  assistant: number;
  system: number;
  'file-history-snapshot': number;
  summary: number;
  'queue-operation': number;
}

/**
 * Get the category for a transcript line (direct mapping from type)
 */
export function categorizeMessage(line: TranscriptLine): MessageCategory {
  return line.type;
}

/**
 * Count messages in each category
 */
export function countCategories(messages: TranscriptLine[]): MessageCategoryCounts {
  const counts: MessageCategoryCounts = {
    user: 0,
    assistant: 0,
    system: 0,
    'file-history-snapshot': 0,
    summary: 0,
    'queue-operation': 0,
  };

  for (const message of messages) {
    counts[message.type]++;
  }

  return counts;
}
