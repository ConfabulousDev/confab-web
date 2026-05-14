// Codex-specific timeline-segment computation + layout hook.
//
// Codex's render-item stream already carries explicit turn boundaries
// (`CodexTurnSeparatorItem` with `durationMs` precomputed), so segment
// derivation here is just "slice on each separator and tag with its
// turnIndex". The layout math (sizing blend, height percents, indicator
// position) is shared with Claude via `useBlendedSegmentLayout`.

import { useMemo } from 'react';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import {
  type BlendedSegment,
  type BlendedSegmentLayout,
  useBlendedSegmentLayout,
} from '../timelineSegments';

/**
 * One segment per agent turn. `startIndex` / `endIndex` are inclusive
 * indices into the `CodexRenderItem[]` the segment was derived from.
 * `messageCount` counts every render item in the turn — user, assistant,
 * tool_call, reasoning_hidden, AND the trailing separator — to mirror
 * Claude's tooltip math.
 */
export interface CodexTimelineSegment extends BlendedSegment {
  turnIndex: number;
  timeToFirstTokenMs?: number;
}

/**
 * Slice the render-item stream on each `CodexTurnSeparatorItem`. Items
 * after the last separator form a trailing in-flight segment with no
 * turnIndex/duration (we synthesize one).
 *
 * The trailing in-flight segment uses a synthetic turnIndex (lastTurnIndex+1
 * or 1 if no separators), durationMs=0, and no TTFT. The bar renders it
 * the same way as any other segment so an in-flight session still has
 * something clickable.
 */
export function computeCodexSegments(items: CodexRenderItem[]): CodexTimelineSegment[] {
  if (items.length === 0) return [];

  const segments: CodexTimelineSegment[] = [];
  let segmentStart = 0;
  let lastTurnIndex = 0;

  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (!item || item.kind !== 'turn_separator') continue;

    segments.push({
      turnIndex: item.turnIndex,
      durationMs: item.durationMs,
      timeToFirstTokenMs: item.timeToFirstTokenMs,
      startIndex: segmentStart,
      endIndex: i,
      messageCount: i - segmentStart + 1,
    });
    lastTurnIndex = item.turnIndex;
    segmentStart = i + 1;
  }

  // Trailing in-flight segment (items after the last separator, or all
  // items if there are no separators yet).
  if (segmentStart < items.length) {
    segments.push({
      turnIndex: lastTurnIndex + 1,
      durationMs: 0,
      timeToFirstTokenMs: undefined,
      startIndex: segmentStart,
      endIndex: items.length - 1,
      messageCount: items.length - segmentStart,
    });
  }

  return segments;
}

/**
 * Codex analog of `useSegmentLayout`. Computes segments from the render
 * items and delegates the size/position math to `useBlendedSegmentLayout`.
 */
export function useCodexSegmentLayout(
  items: CodexRenderItem[],
  selectedIndex: number,
): BlendedSegmentLayout<CodexTimelineSegment> {
  const segments = useMemo(() => computeCodexSegments(items), [items]);
  return useBlendedSegmentLayout(segments, selectedIndex);
}
