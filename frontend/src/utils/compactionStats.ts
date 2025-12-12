import type { TranscriptLine } from '@/types';

/**
 * Format milliseconds as a human-readable duration string.
 * Examples: "4.2s", "1m 23s", "2h 15m"
 */
export function formatResponseTime(ms: number | null): string {
  if (ms === null) {
    return '-';
  }

  // Round to nearest second first to avoid "1m 60s" edge cases
  const totalSeconds = Math.round(ms / 1000);

  if (totalSeconds < 60) {
    // For sub-minute, show one decimal place from original ms
    const seconds = ms / 1000;
    return `${seconds.toFixed(1)}s`;
  }

  const minutes = Math.floor(totalSeconds / 60);
  const remainingSeconds = totalSeconds % 60;

  if (minutes < 60) {
    return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
}

export interface CompactionStats {
  total: number;
  auto: number;
  manual: number;
  avgCompactionTimeMs: number | null; // Average time for compaction (logicalParent → compact_boundary)
}

/**
 * Calculate compaction stats by counting compact_boundary system messages.
 * Auto + Manual should equal Total.
 *
 * Also calculates average compaction time: the time from the last message
 * before compaction (logicalParentUuid) to the compact_boundary timestamp.
 * This measures actual server-side summarization time.
 */
export function calculateCompactionStats(messages: TranscriptLine[]): CompactionStats {
  const stats: CompactionStats = {
    total: 0,
    auto: 0,
    manual: 0,
    avgCompactionTimeMs: null,
  };

  // Build a map of uuid → timestamp for quick lookup
  const timestampByUuid = new Map<string, string>();
  for (const message of messages) {
    if ('uuid' in message && 'timestamp' in message) {
      timestampByUuid.set(message.uuid, message.timestamp);
    }
  }

  const compactionTimes: number[] = [];

  for (const message of messages) {
    if (message.type !== 'system' || message.subtype !== 'compact_boundary') {
      continue;
    }

    stats.total++;
    const trigger = message.compactMetadata?.trigger;
    if (trigger === 'auto') {
      stats.auto++;
    } else if (trigger === 'manual') {
      stats.manual++;
    }

    // Calculate compaction time: logicalParent timestamp → compact_boundary timestamp
    const logicalParentUuid = message.logicalParentUuid;
    if (logicalParentUuid) {
      const parentTimestamp = timestampByUuid.get(logicalParentUuid);
      if (parentTimestamp) {
        const startTime = new Date(parentTimestamp).getTime();
        const endTime = new Date(message.timestamp).getTime();
        const delta = endTime - startTime;
        if (delta >= 0) {
          compactionTimes.push(delta);
        }
      }
    }
  }

  if (compactionTimes.length > 0) {
    const sum = compactionTimes.reduce((a, b) => a + b, 0);
    stats.avgCompactionTimeMs = Math.round(sum / compactionTimes.length);
  }

  return stats;
}
