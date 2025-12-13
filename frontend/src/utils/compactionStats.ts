import type { TranscriptLine } from '@/types';
import { formatDuration } from './formatting';

/**
 * Format milliseconds as a human-readable duration string, or '-' for null.
 * Uses decimal seconds for precision (e.g., "4.2s", "1m 23s", "2h 15m")
 */
export function formatResponseTime(ms: number | null): string {
  if (ms === null) {
    return '-';
  }
  return formatDuration(ms, { decimalSeconds: true });
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
 * Also calculates average compaction time for AUTO compactions only:
 * the time from the last message before compaction (logicalParentUuid)
 * to the compact_boundary timestamp. This measures actual server-side
 * summarization time.
 *
 * Manual compactions are excluded from timing because the /compact command
 * is not recorded in the transcript, so the gap includes arbitrary user
 * think time rather than actual processing time.
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

    // Calculate compaction time only for auto compactions
    // Manual compactions include user think time, not just processing time
    if (trigger === 'auto') {
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
  }

  if (compactionTimes.length > 0) {
    const sum = compactionTimes.reduce((a, b) => a + b, 0);
    stats.avgCompactionTimeMs = Math.round(sum / compactionTimes.length);
  }

  return stats;
}
