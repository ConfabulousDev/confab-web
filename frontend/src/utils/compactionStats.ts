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
  avgResponseTimeMs: number | null; // Average time from compact_boundary to first assistant response
}

/**
 * Calculate compaction stats by counting compact_boundary system messages.
 * Auto + Manual should equal Total.
 *
 * Also calculates average response time: the time from compact_boundary
 * to the first assistant message after it.
 */
export function calculateCompactionStats(messages: TranscriptLine[]): CompactionStats {
  const stats: CompactionStats = {
    total: 0,
    auto: 0,
    manual: 0,
    avgResponseTimeMs: null,
  };

  const responseTimes: number[] = [];

  for (const [i, message] of messages.entries()) {
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

    // Find the first assistant message after this compact_boundary
    const boundaryTime = new Date(message.timestamp).getTime();
    const nextAssistant = messages.slice(i + 1).find((m) => m.type === 'assistant');
    if (nextAssistant) {
      const delta = new Date(nextAssistant.timestamp).getTime() - boundaryTime;
      if (delta >= 0) {
        responseTimes.push(delta);
      }
    }
  }

  if (responseTimes.length > 0) {
    const sum = responseTimes.reduce((a, b) => a + b, 0);
    stats.avgResponseTimeMs = Math.round(sum / responseTimes.length);
  }

  return stats;
}
