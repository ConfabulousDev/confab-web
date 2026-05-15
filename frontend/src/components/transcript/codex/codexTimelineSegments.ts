// Codex timeline segments: each completed turn emits up to two segments —
// a user thinking-gap stripe and an assistant body stripe. Turn 1 has no
// prior separator, so its user gap is a synthetic 1s; zero/negative
// computed gaps clamp to the same 1s floor so the stripe stays clickable.
// Slices with no user item (compaction-only) collapse to one assistant
// segment. Layout (sizing, indicator) is shared via useBlendedSegmentLayout.

import { useMemo } from 'react';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import {
  type BlendedSegment,
  type BlendedSegmentLayout,
  useBlendedSegmentLayout,
} from '../timelineSegments';

export type CodexSpeaker = 'user' | 'assistant';

export interface CodexTimelineSegment extends BlendedSegment {
  speaker: CodexSpeaker;
  turnIndex: number;
}

/** Floor for user thinking-gap duration (also used for turn 1's synthetic gap). */
const FIRST_TURN_USER_SEGMENT_MS = 1000;

export function computeCodexSegments(items: CodexRenderItem[]): CodexTimelineSegment[] {
  if (items.length === 0) return [];

  const segments: CodexTimelineSegment[] = [];
  let sliceStart = 0;
  let lastTurnIndex = 0;
  let prevSeparatorTimestamp: string | null = null;

  const pushTurn = (
    turnIndex: number,
    start: number,
    end: number,
    bodyDurationMs: number,
  ): void => {
    const userIdx = findUserIndex(items, start, end);

    if (userIdx === -1) {
      segments.push({
        speaker: 'assistant',
        turnIndex,
        durationMs: bodyDurationMs,
        startIndex: start,
        endIndex: end,
        messageCount: end - start + 1,
      });
      return;
    }

    segments.push({
      speaker: 'user',
      turnIndex,
      durationMs: computeUserDurationMs(items[userIdx]?.timestamp, prevSeparatorTimestamp),
      startIndex: userIdx,
      endIndex: userIdx,
      messageCount: 1,
    });

    if (hasNonSeparatorContent(items, userIdx + 1, end)) {
      segments.push({
        speaker: 'assistant',
        turnIndex,
        durationMs: bodyDurationMs,
        startIndex: userIdx + 1,
        endIndex: end,
        messageCount: end - userIdx,
      });
    }
  };

  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (!item || item.kind !== 'turn_separator') continue;

    pushTurn(item.turnIndex, sliceStart, i, item.durationMs);
    lastTurnIndex = item.turnIndex;
    prevSeparatorTimestamp = item.timestamp;
    sliceStart = i + 1;
  }

  // Trailing in-flight slice (items past the last separator).
  if (sliceStart < items.length) {
    pushTurn(lastTurnIndex + 1, sliceStart, items.length - 1, 0);
  }

  return segments;
}

function findUserIndex(items: CodexRenderItem[], start: number, end: number): number {
  for (let i = start; i <= end; i++) {
    if (items[i]?.kind === 'user') return i;
  }
  return -1;
}

function hasNonSeparatorContent(items: CodexRenderItem[], start: number, end: number): boolean {
  for (let i = start; i <= end; i++) {
    if (items[i]?.kind !== 'turn_separator') return true;
  }
  return false;
}

function computeUserDurationMs(
  userTimestamp: string | undefined,
  prevSeparatorTimestamp: string | null,
): number {
  if (!userTimestamp || !prevSeparatorTimestamp) return FIRST_TURN_USER_SEGMENT_MS;
  const delta = new Date(userTimestamp).getTime() - new Date(prevSeparatorTimestamp).getTime();
  if (!Number.isFinite(delta) || delta <= 0) return FIRST_TURN_USER_SEGMENT_MS;
  return delta;
}

export function useCodexSegmentLayout(
  items: CodexRenderItem[],
  selectedIndex: number,
): BlendedSegmentLayout<CodexTimelineSegment> {
  const segments = useMemo(() => computeCodexSegments(items), [items]);
  return useBlendedSegmentLayout(segments, selectedIndex);
}
