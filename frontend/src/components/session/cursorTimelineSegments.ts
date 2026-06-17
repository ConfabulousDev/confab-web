// Cursor timeline segments (zztp): synthesizes turns from `user`-item
// transitions (there is NO turn_separator in the Cursor render-item stream,
// exactly like OpenCode). Each user item opens a turn and emits a user
// thinking-gap stripe; the non-user body items after it (up to the next user)
// fold into one assistant stripe (tools included — 2-color bar). Leading
// non-user items before the first user collapse to a single assistant segment.
// Layout (sizing, indicator) is shared via useBlendedSegmentLayout.
//
// CORRECTNESS (differs from OpenCode's `timeCreated`): Cursor's per-row
// `timestamp` is an OPTIONAL ISO-8601 STRING, estimated frontend-side over the
// session bounds (ce79) — never sourced from the wire. We parse it to epoch-ms
// with `Date.parse`, so all durations are numeric subtraction on the parsed
// values. Per ce79's "no bogus times" stance, if ANY item lacks a parseable
// timestamp (bounds unknown), the whole stream yields ZERO segments so the
// shared TimelineBar self-hides — we never draw a bar over invented times.

import { useMemo } from 'react';
import type { CursorRenderItem } from './cursorCategories';
import {
  type BlendedSegmentLayout,
  type SpeakerSegment,
  useBlendedSegmentLayout,
} from '@/components/transcript/timelineSegments';

export type CursorTimelineSegment = SpeakerSegment;

/** Floor for user thinking-gap duration (also used for turn 1 / non-positive gaps). */
const FIRST_TURN_USER_SEGMENT_MS = 1000;
/** Floor for assistant body duration (clamps zero/negative computed bodies). */
const MIN_ASSISTANT_SEGMENT_MS = 1000;

export function computeCursorSegments(
  items: CursorRenderItem[],
): CursorTimelineSegment[] {
  if (items.length === 0) return [];

  // ce79 "no bogus times": estimated timestamps are all-or-nothing (absent
  // until session bounds are known). If any row lacks a parseable timestamp,
  // synthesize no segments → the shared bar hides rather than draw over
  // invented times.
  const timeMs: number[] = [];
  for (const item of items) {
    const ms = item.timestamp ? Date.parse(item.timestamp) : NaN;
    if (Number.isNaN(ms)) return [];
    timeMs.push(ms);
  }

  const segments: CursorTimelineSegment[] = [];
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
    segments.push(makeAssistantSegment(timeMs, 0, items.length - 1, turnIndex));
    return segments;
  }
  if (firstUser > 0) {
    turnIndex += 1;
    segments.push(makeAssistantSegment(timeMs, 0, firstUser - 1, turnIndex));
  }

  for (let k = 0; k < userIndices.length; k++) {
    const userIdx = userIndices[k]!;
    const nextUserIdx = userIndices[k + 1] ?? items.length;
    const bodyEnd = nextUserIdx - 1; // inclusive, may equal userIdx (no body)
    turnIndex += 1;

    // User thinking-gap: time from the previous turn's last item to this user.
    // `prevLastMs` is the item immediately before this user (the tail of the
    // prior turn's body, or the prior user for back-to-back users).
    const userMs = timeMs[userIdx]!;
    const prevLastMs = userIdx > 0 ? timeMs[userIdx - 1] : undefined;
    segments.push({
      speaker: 'user',
      turnIndex,
      durationMs: computeGapMs(prevLastMs, userMs),
      startIndex: userIdx,
      endIndex: userIdx,
      messageCount: 1,
    });

    // Assistant body: the non-user items after the user, up to the next user.
    if (bodyEnd > userIdx) {
      segments.push(makeAssistantSegment(timeMs, userIdx + 1, bodyEnd, turnIndex, userMs));
    }
  }

  return segments;
}

function makeAssistantSegment(
  timeMs: number[],
  start: number,
  end: number,
  turnIndex: number,
  bodyAnchorMs?: number,
): CursorTimelineSegment {
  // Body duration = last body item's time minus the anchor (the user's time
  // when present, else the first body item's time), floored at 1s.
  const anchor = bodyAnchorMs ?? timeMs[start];
  const lastBody = timeMs[end];
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

export function useCursorSegmentLayout(
  items: CursorRenderItem[],
  selectedIndex: number,
): BlendedSegmentLayout<CursorTimelineSegment> {
  const segments = useMemo(() => computeCursorSegments(items), [items]);
  return useBlendedSegmentLayout(segments, selectedIndex);
}
