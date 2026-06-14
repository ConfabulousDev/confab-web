// OpenCode timeline segments: synthesizes turns from `user`-item transitions
// (there is NO turn_separator in the OpenCode render-item stream). Each user
// item opens a turn and emits a user thinking-gap stripe; the non-user body
// items after it (up to the next user) fold into one assistant stripe (tools
// included — 2-color bar). Leading non-user items before the first user
// collapse to a single assistant segment. Layout (sizing, indicator) is
// shared via useBlendedSegmentLayout.
//
// CORRECTNESS: `timeCreated` is epoch-ms (a NUMBER), so all durations are
// plain numeric subtraction (`b.timeCreated - a.timeCreated`). Do NOT wrap in
// `new Date(...)` — that path expects a date string and would NaN here.

import { useMemo } from 'react';
import type { OpenCodeRenderItem } from './opencodeCategories';
import {
  type BlendedSegmentLayout,
  type SpeakerSegment,
  useBlendedSegmentLayout,
} from '@/components/transcript/timelineSegments';

export type OpenCodeTimelineSegment = SpeakerSegment;

/** Floor for user thinking-gap duration (also used for turn 1 / non-positive gaps). */
const FIRST_TURN_USER_SEGMENT_MS = 1000;
/** Floor for assistant body duration (clamps zero/negative computed bodies). */
const MIN_ASSISTANT_SEGMENT_MS = 1000;

export function computeOpenCodeSegments(
  items: OpenCodeRenderItem[],
): OpenCodeTimelineSegment[] {
  if (items.length === 0) return [];

  const segments: OpenCodeTimelineSegment[] = [];
  let turnIndex = 0;

  // Find the indices of all user items — each opens a turn.
  const userIndices: number[] = [];
  for (let i = 0; i < items.length; i++) {
    if (items[i]?.kind === 'user') userIndices.push(i);
  }

  // Leading non-user items before the first user → one assistant segment.
  const firstUser = userIndices[0];
  if (firstUser === undefined) {
    // No user items at all: the whole stream is one assistant body.
    turnIndex += 1;
    segments.push(makeAssistantSegment(items, 0, items.length - 1, turnIndex));
    return segments;
  }
  if (firstUser > 0) {
    turnIndex += 1;
    segments.push(makeAssistantSegment(items, 0, firstUser - 1, turnIndex));
  }

  for (let k = 0; k < userIndices.length; k++) {
    const userIdx = userIndices[k]!;
    const nextUserIdx = userIndices[k + 1] ?? items.length;
    const bodyEnd = nextUserIdx - 1; // inclusive, may equal userIdx (no body)
    turnIndex += 1;

    // User thinking-gap: time from the previous turn's last item to this user.
    // `prevLastItem` is the item immediately before this user (the tail of the
    // prior turn's body, or the prior user for back-to-back users).
    const user = items[userIdx]!;
    const prevLastItem = userIdx > 0 ? items[userIdx - 1] : undefined;
    const userDurationMs = computeGapMs(prevLastItem?.timeCreated, user.timeCreated);
    segments.push({
      speaker: 'user',
      turnIndex,
      durationMs: userDurationMs,
      startIndex: userIdx,
      endIndex: userIdx,
      messageCount: 1,
    });

    // Assistant body: the non-user items after the user, up to the next user.
    if (bodyEnd > userIdx) {
      segments.push(makeAssistantSegment(items, userIdx + 1, bodyEnd, turnIndex, user.timeCreated));
    }
  }

  return segments;
}

function makeAssistantSegment(
  items: OpenCodeRenderItem[],
  start: number,
  end: number,
  turnIndex: number,
  bodyAnchorMs?: number,
): OpenCodeTimelineSegment {
  // Body duration = last body item's time minus the anchor (the user's time
  // when present, else the first body item's time), floored at 1s.
  const anchor = bodyAnchorMs ?? items[start]?.timeCreated;
  const lastBody = items[end]?.timeCreated;
  let durationMs = MIN_ASSISTANT_SEGMENT_MS;
  if (anchor !== undefined && lastBody !== undefined) {
    const delta = lastBody - anchor;
    durationMs = Number.isFinite(delta) && delta > 0 ? delta : MIN_ASSISTANT_SEGMENT_MS;
  }
  return {
    speaker: 'assistant',
    turnIndex,
    durationMs,
    startIndex: start,
    endIndex: end,
    messageCount: end - start + 1,
  };
}

function computeGapMs(prevMs: number | undefined, userMs: number): number {
  if (prevMs === undefined) return FIRST_TURN_USER_SEGMENT_MS;
  const delta = userMs - prevMs;
  if (!Number.isFinite(delta) || delta <= 0) return FIRST_TURN_USER_SEGMENT_MS;
  return delta;
}

export function useOpenCodeSegmentLayout(
  items: OpenCodeRenderItem[],
  selectedIndex: number,
): BlendedSegmentLayout<OpenCodeTimelineSegment> {
  const segments = useMemo(() => computeOpenCodeSegments(items), [items]);
  return useBlendedSegmentLayout(segments, selectedIndex);
}
