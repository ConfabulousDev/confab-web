// Spec tests for Cursor timeline-segment computation (zztp).
//
// Cursor has NO turn_separator: turns are synthesized from `user`-item
// transitions, exactly like OpenCode. Each user opens a turn → a user
// thinking-gap stripe (1s floor for turn 1 / non-positive gaps) plus an
// assistant body stripe folding all non-user body items (tools included).
// Leading non-user items before the first user collapse to one assistant
// segment.
//
// CORRECTNESS TRAP (differs from OpenCode): Cursor `timestamp` is an OPTIONAL
// ISO-8601 STRING, not epoch-ms. Durations come from `Date.parse(timestamp)`.
// When ANY item lacks a `timestamp` (bounds unknown — ce79's "no bogus times"
// stance), the whole computation returns an empty array so the bar self-hides.

import { describe, it, expect } from 'vitest';
import { computeCursorSegments } from './cursorTimelineSegments';
import type { CursorRenderItem } from './cursorCategories';

const T0 = Date.parse('2026-06-17T10:00:00.000Z');
const iso = (offsetMs: number) => new Date(T0 + offsetMs).toISOString();

function user(offsetMs: number, text = 'hi'): CursorRenderItem {
  return { kind: 'user', id: `u${offsetMs}`, text, timestamp: iso(offsetMs) };
}
function assistant(offsetMs: number, text = 'hello'): CursorRenderItem {
  return { kind: 'assistant', id: `a${offsetMs}`, text, timestamp: iso(offsetMs) };
}
function tool(offsetMs: number, toolName = 'Read'): CursorRenderItem {
  return { kind: 'tool', id: `t${offsetMs}`, toolName, timestamp: iso(offsetMs) };
}

describe('computeCursorSegments', () => {
  it('returns an empty array for empty items', () => {
    expect(computeCursorSegments([])).toEqual([]);
  });

  it('emits a user + assistant segment for a single turn', () => {
    const items = [user(0), assistant(5000)];
    const segments = computeCursorSegments(items);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toMatchObject({
      speaker: 'user',
      turnIndex: 1,
      durationMs: 1000, // first turn → 1s floor
      startIndex: 0,
      endIndex: 0,
      messageCount: 1,
    });
    expect(segments[1]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      durationMs: 5000, // last body (T0+5000) - user (T0)
      startIndex: 1,
      endIndex: 1,
      messageCount: 1,
    });
  });

  it('uses the wall-clock gap from the prior turn for turn 2 user duration', () => {
    const items = [
      user(0),
      assistant(5000),
      user(95_000), // 90s after the prior assistant (T0+5000)
      assistant(100_000),
    ];
    const segments = computeCursorSegments(items);
    expect(segments).toHaveLength(4);
    expect(segments[2]).toMatchObject({
      speaker: 'user',
      turnIndex: 2,
      durationMs: 90_000,
      startIndex: 2,
      endIndex: 2,
      messageCount: 1,
    });
    expect(segments[3]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 2,
      durationMs: 5000,
      startIndex: 3,
      endIndex: 3,
      messageCount: 1,
    });
  });

  it('folds a tool-heavy turn into a single assistant stripe', () => {
    const items = [
      user(0),
      assistant(1000),
      tool(2000),
      tool(3000),
      assistant(4000),
    ];
    const segments = computeCursorSegments(items);
    expect(segments).toHaveLength(2);
    expect(segments[1]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      startIndex: 1,
      endIndex: 4,
      messageCount: 4, // assistant + tool + tool + assistant all fold in
      durationMs: 4000, // last body (T0+4000) - user (T0)
    });
  });

  it('clamps zero / negative user gaps to the 1s floor', () => {
    const items = [
      user(0),
      assistant(1000),
      user(1000), // same instant as the prior assistant → 0ms gap
      assistant(2000),
    ];
    const segments = computeCursorSegments(items);
    const turn2User = segments.find((s) => s.turnIndex === 2 && s.speaker === 'user');
    expect(turn2User?.durationMs).toBe(1000);
  });

  it('clamps zero / negative assistant body durations to the 1s floor', () => {
    const items = [user(0), assistant(0)]; // body item at same instant as user
    const segments = computeCursorSegments(items);
    expect(segments[1]).toMatchObject({ speaker: 'assistant', durationMs: 1000 });
  });

  it('collapses leading non-user items into a single assistant segment', () => {
    const items = [
      assistant(0),
      tool(1000),
      user(2000),
      assistant(3000),
    ];
    const segments = computeCursorSegments(items);
    // [leading assistant, turn1 user, turn1 assistant]
    expect(segments).toHaveLength(3);
    expect(segments[0]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      startIndex: 0,
      endIndex: 1,
      messageCount: 2,
    });
    expect(segments[1]).toMatchObject({ speaker: 'user', turnIndex: 2, startIndex: 2 });
    expect(segments[2]).toMatchObject({ speaker: 'assistant', turnIndex: 2, startIndex: 3 });
  });

  it('emits a single assistant segment when there are no user items', () => {
    const items = [assistant(0), tool(1000)];
    const segments = computeCursorSegments(items);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      startIndex: 0,
      endIndex: 1,
      messageCount: 2,
    });
  });

  it('emits only a user segment for a user-only (body-less) trailing turn', () => {
    const items = [user(0), assistant(1000), user(2000)];
    const segments = computeCursorSegments(items);
    // [t1 user, t1 assistant, t2 user] — t2 has no body
    expect(segments).toHaveLength(3);
    expect(segments[2]).toMatchObject({ speaker: 'user', turnIndex: 2, startIndex: 2, messageCount: 1 });
  });

  // D2: ce79's "no bogus times" stance — when timestamps are absent (bounds
  // unknown), the bar must hide. A missing timestamp on ANY item collapses the
  // whole layout to empty (TimelineBar self-hides at zero segments).
  it('returns an empty array when items have no timestamps (bar hidden)', () => {
    const items: CursorRenderItem[] = [
      { kind: 'user', id: '0', text: 'hi' },
      { kind: 'assistant', id: '1', text: 'hello' },
    ];
    expect(computeCursorSegments(items)).toEqual([]);
  });

  it('returns an empty array when a single item is missing a timestamp', () => {
    const items: CursorRenderItem[] = [
      user(0),
      { kind: 'assistant', id: 'a-no-ts', text: 'hello' }, // no timestamp
    ];
    expect(computeCursorSegments(items)).toEqual([]);
  });

  it('returns an empty array when a timestamp is unparseable', () => {
    const items: CursorRenderItem[] = [
      { kind: 'user', id: '0', text: 'hi', timestamp: 'not-a-date' },
      assistant(1000),
    ];
    expect(computeCursorSegments(items)).toEqual([]);
  });

  it('CORRECTNESS TRAP: durations parse ISO strings, not epoch-ms numbers', () => {
    // A 12.345s assistant body, encoded as ISO strings. The delta must be the
    // exact parsed difference, never NaN (which a number-subtraction path on
    // strings would produce).
    const items = [user(0), assistant(12_345)];
    const segments = computeCursorSegments(items);
    const assistantSeg = segments.find((s) => s.speaker === 'assistant');
    expect(assistantSeg?.durationMs).toBe(12_345);
    expect(Number.isNaN(assistantSeg?.durationMs)).toBe(false);
  });
});
