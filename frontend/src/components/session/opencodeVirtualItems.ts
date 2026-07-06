// Pure builder for the OpenCode virtual-item layer (real rows + injected
// time/day separators). Mirrors codexVirtualItems.ts / claudeVirtualItems.ts /
// cursorVirtualItems.ts (6h7m). OpenCode's `timeCreated` is a REAL, required
// epoch-ms field (`info.time.created`) on every render-item variant — no
// estimation involved (unlike Cursor), so no separator is ever flagged
// `estimated`.

import type { OpenCodeRenderItem } from './opencodeCategories';
import { shouldShowDivider, formatDividerLabel } from '@/components/transcript/timelineUtils';

/** Virtual-list item layer: real OpenCode rows + injected time/day separators. */
export type VirtualItem =
  | { type: 'item'; item: OpenCodeRenderItem; index: number }
  | { type: 'separator'; label: string };

/**
 * Build the virtual-list layer over an OpenCode render-item stream (the pane
 * passes its FILTERED items — `index` on each entry is that array's index,
 * matching the existing filtered-index axis skip-nav/search already use).
 */
export function buildVirtualItems(items: OpenCodeRenderItem[]): VirtualItem[] {
  const out: VirtualItem[] = [];
  let lastKnownMs: number | undefined;

  items.forEach((item, index) => {
    const currentMs = item.timeCreated;
    const { show, dayChanged } = shouldShowDivider(currentMs, lastKnownMs);
    if (show) {
      out.push({ type: 'separator', label: formatDividerLabel(currentMs, dayChanged) });
    }
    lastKnownMs = currentMs;

    out.push({ type: 'item', item, index });
  });

  return out;
}
