// Pure builder for the Claude virtual-item layer (real messages + injected
// time/day separators). Extracted from ClaudeMessageTimeline.tsx (6h7m) to
// mirror codexVirtualItems.ts: unit-testable without spinning up the
// virtualizer, and keeps ClaudeMessageTimeline.tsx react-refresh-clean.

import type { TranscriptLine } from '@/types';
import { shouldShowDivider, formatDividerLabel } from '@/components/transcript/timelineUtils';

/** Virtual-list item layer: real messages + injected time/day separators. */
export type VirtualItem =
  | { type: 'message'; message: TranscriptLine; index: number; filteredIndex: number }
  | { type: 'separator'; label: string };

/**
 * Epoch ms for a `TranscriptLine`'s timestamp, or `undefined` if it has none.
 * Only some `TranscriptLine` variants carry `timestamp` (e.g. `summary` lines
 * do not — see `SummaryMessageSchema`), so callers must guard with `in`.
 */
function timestampMs(message: TranscriptLine): number | undefined {
  if (!('timestamp' in message) || typeof message.timestamp !== 'string') return undefined;
  const ms = new Date(message.timestamp).getTime();
  return Number.isNaN(ms) ? undefined : ms;
}

/**
 * Build the virtual-list layer: filtered messages + an injected divider row
 * wherever the shared `shouldShowDivider` predicate fires (day change OR
 * >5min gap — 6h7m unifies the day-boundary and idle-gap dividers into one).
 *
 * Compares each timestamped message against the last message that HAD a
 * timestamp, not simply the immediately-previous array element (Decision 7),
 * so a timestamp-less line (e.g. Claude's `summary`) sitting between two real
 * timestamps doesn't silently swallow a real day/gap boundary.
 *
 * `messageToAllIndex` maps a message to its index in the (unfiltered)
 * `allMessages` array, for `TimelineBar` compatibility — mirrors the caller's
 * pre-extraction behavior.
 */
export function buildVirtualItems(
  messages: TranscriptLine[],
  messageToAllIndex: Map<TranscriptLine, number>,
): VirtualItem[] {
  const items: VirtualItem[] = [];
  let lastKnownMs: number | undefined;

  messages.forEach((message, filteredIndex) => {
    const currentMs = timestampMs(message);

    if (currentMs !== undefined) {
      const { show, dayChanged } = shouldShowDivider(currentMs, lastKnownMs);
      if (show) {
        items.push({ type: 'separator', label: formatDividerLabel(currentMs, dayChanged) });
      }
      lastKnownMs = currentMs;
    }

    // Use allMessages index for TimelineBar compatibility.
    const allIndex = messageToAllIndex.get(message) ?? filteredIndex;
    items.push({ type: 'message', message, index: allIndex, filteredIndex });
  });

  return items;
}
