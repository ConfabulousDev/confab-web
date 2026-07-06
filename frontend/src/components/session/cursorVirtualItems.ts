// Pure builder for the Cursor virtual-item layer (real rows + injected
// time/day separators). Mirrors codexVirtualItems.ts / claudeVirtualItems.ts
// (6h7m). Cursor's per-row `timestamp` is OPTIONAL and ESTIMATED (ce79) —
// interpolated frontend-side over the session's firstSeen/lastSyncAt bounds,
// never sourced from the wire — so every divider this builder injects is
// flagged `estimated: true` (Decision 8) rather than suppressed, matching the
// `~`-prefix/tooltip convention used elsewhere on Cursor rows. When bounds
// are unknown, every item's `timestamp` is `undefined`, so `shouldShowDivider`
// never fires and this builder synthesizes zero dividers — the same
// fail-safe stance as `cursorTimelineSegments.ts`'s "no bogus times" check.

import type { CursorRenderItem } from './cursorCategories';
import { shouldShowDivider, formatDividerLabel } from '@/components/transcript/timelineUtils';

/** Virtual-list item layer: real Cursor rows + injected time/day separators. */
export type VirtualItem =
  | { type: 'item'; item: CursorRenderItem; index: number }
  | { type: 'separator'; label: string; estimated: true };

/** Epoch ms for a Cursor row's (optional, estimated) timestamp. */
function timestampMs(item: CursorRenderItem): number | undefined {
  if (!item.timestamp) return undefined;
  const ms = Date.parse(item.timestamp);
  return Number.isNaN(ms) ? undefined : ms;
}

/**
 * Build the virtual-list layer over a Cursor render-item stream (the pane
 * passes its FILTERED, timestamp-stamped items — `index` on each entry is
 * that array's index, matching the existing filtered-index axis skip-nav and
 * search already use).
 */
export function buildVirtualItems(items: CursorRenderItem[]): VirtualItem[] {
  const out: VirtualItem[] = [];
  let lastKnownMs: number | undefined;

  items.forEach((item, index) => {
    const currentMs = timestampMs(item);
    if (currentMs !== undefined) {
      const { show, dayChanged } = shouldShowDivider(currentMs, lastKnownMs);
      if (show) {
        out.push({
          type: 'separator',
          label: formatDividerLabel(currentMs, dayChanged),
          estimated: true,
        });
      }
      lastKnownMs = currentMs;
    }

    out.push({ type: 'item', item, index });
  });

  return out;
}
