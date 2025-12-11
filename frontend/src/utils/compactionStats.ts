import type { TranscriptLine } from '@/types';

export interface CompactionStats {
  total: number;
  auto: number;
  manual: number;
}

/**
 * Calculate compaction stats by counting compact_boundary system messages.
 * Auto + Manual should equal Total.
 */
export function calculateCompactionStats(messages: TranscriptLine[]): CompactionStats {
  const stats: CompactionStats = {
    total: 0,
    auto: 0,
    manual: 0,
  };

  for (const message of messages) {
    if (message.type === 'system' && message.subtype === 'compact_boundary') {
      stats.total++;
      const trigger = message.compactMetadata?.trigger;
      if (trigger === 'auto') {
        stats.auto++;
      } else if (trigger === 'manual') {
        stats.manual++;
      }
      // If trigger is missing or unknown, it still counts toward total
    }
  }

  return stats;
}
