// Pure builder for the Codex virtual-item layer (real items + injected
// time-gap separators). Lives in its own file so it can be unit-tested
// without spinning up the virtualizer, and so `CodexMessageTimeline.tsx`
// satisfies react-refresh's "only-export-components" rule.

import type { CodexRenderItem } from '@/types/codexRenderItem';

/** Virtual-list item layer: real Codex items + injected time separators. */
export type VirtualItem =
  | { type: 'item'; item: CodexRenderItem; index: number; isNewSpeaker: boolean }
  | { type: 'separator'; timestamp: string };

/** Mirrors MessageTimeline.tsx's 5-min threshold for time-gap dividers. */
const TIME_GAP_THRESHOLD_MS = 5 * 60 * 1000;

/** True iff `>5 min` between consecutive items' timestamps. */
function shouldShowTimeSeparator(
  current: CodexRenderItem,
  previous: CodexRenderItem | undefined,
): boolean {
  if (!previous) return false;
  const currentTime = new Date(current.timestamp);
  const previousTime = new Date(previous.timestamp);
  if (Number.isNaN(currentTime.getTime()) || Number.isNaN(previousTime.getTime())) return false;
  return currentTime.getTime() - previousTime.getTime() > TIME_GAP_THRESHOLD_MS;
}

/**
 * Build the virtual-list layer from a render-item stream:
 *   - inject a time separator before any item whose timestamp is >5min after
 *     the previous item's,
 *   - tag every item with `isNewSpeaker` per the speaker-continuity rule.
 *
 * Speaker rule: track the last user/assistant kind seen. Mark the current
 * item as newSpeaker iff its kind is user|assistant AND a previous speaker
 * exists AND the previous speaker kind differs. tool_call, reasoning_hidden,
 * turn_separator, compacted, and unknown items do NOT update the tracked
 * speaker — so user → tool_call → user is the same speaker (Codex-specific
 * carveout; Claude has no analog because tool_use is a content block, not
 * a separate timeline item).
 */
export function buildVirtualItems(items: CodexRenderItem[]): VirtualItem[] {
  const out: VirtualItem[] = [];
  let lastSpeaker: 'user' | 'assistant' | null = null;
  let prev: CodexRenderItem | undefined;

  items.forEach((item, index) => {
    if (shouldShowTimeSeparator(item, prev)) {
      out.push({ type: 'separator', timestamp: item.timestamp });
    }

    const isNewSpeaker =
      (item.kind === 'user' || item.kind === 'assistant') &&
      lastSpeaker !== null &&
      lastSpeaker !== item.kind;

    if (item.kind === 'user' || item.kind === 'assistant') {
      lastSpeaker = item.kind;
    }

    out.push({ type: 'item', item, index, isNewSpeaker });
    prev = item;
  });

  return out;
}
