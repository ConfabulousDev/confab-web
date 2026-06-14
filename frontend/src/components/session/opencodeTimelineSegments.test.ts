// Spec tests for OpenCode timeline-segment computation.
//
// OpenCode has NO turn_separator: turns are synthesized from `user`-item
// transitions. Each user opens a turn → a user thinking-gap stripe (1s floor
// for turn 1 / non-positive gaps) plus an assistant body stripe folding all
// non-user body items (tools included — 2-color bar). Leading non-user items
// before the first user collapse to one assistant segment.
//
// CORRECTNESS TRAP: durations are plain numeric subtraction on the epoch-ms
// `timeCreated` field — never `new Date(...)`.

import { describe, it, expect } from 'vitest';
import { computeOpenCodeSegments } from './opencodeTimelineSegments';
import type { OpenCodeRenderItem } from './opencodeCategories';

const T0 = 1_717_689_500_000; // arbitrary epoch ms

function user(timeCreated: number, text = 'hi'): OpenCodeRenderItem {
  return { kind: 'user', id: `u${timeCreated}`, text, timeCreated };
}
function assistant(timeCreated: number, text = 'hello'): OpenCodeRenderItem {
  return { kind: 'assistant', id: `a${timeCreated}`, text, timeCreated };
}
function tool(timeCreated: number, toolName = 'Bash'): OpenCodeRenderItem {
  return { kind: 'tool', id: `t${timeCreated}`, toolName, status: 'completed', timeCreated };
}

describe('computeOpenCodeSegments', () => {
  it('returns an empty array for empty items', () => {
    expect(computeOpenCodeSegments([])).toEqual([]);
  });

  it('emits a user + assistant segment for a single turn', () => {
    const items = [user(T0), assistant(T0 + 5000)];
    const segments = computeOpenCodeSegments(items);
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
      user(T0),
      assistant(T0 + 5000),
      user(T0 + 95_000), // 90s after the prior assistant (T0+5000)
      assistant(T0 + 100_000),
    ];
    const segments = computeOpenCodeSegments(items);
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
      user(T0),
      assistant(T0 + 1000),
      tool(T0 + 2000),
      tool(T0 + 3000),
      assistant(T0 + 4000),
    ];
    const segments = computeOpenCodeSegments(items);
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
      user(T0),
      assistant(T0 + 1000),
      user(T0 + 1000), // same instant as the prior assistant → 0ms gap
      assistant(T0 + 2000),
    ];
    const segments = computeOpenCodeSegments(items);
    const turn2User = segments.find((s) => s.turnIndex === 2 && s.speaker === 'user');
    expect(turn2User?.durationMs).toBe(1000);
  });

  it('clamps zero / negative assistant body durations to the 1s floor', () => {
    const items = [user(T0), assistant(T0)]; // body item at same instant as user
    const segments = computeOpenCodeSegments(items);
    expect(segments[1]).toMatchObject({ speaker: 'assistant', durationMs: 1000 });
  });

  it('collapses leading non-user items into a single assistant segment', () => {
    const items = [
      assistant(T0),
      tool(T0 + 1000),
      user(T0 + 2000),
      assistant(T0 + 3000),
    ];
    const segments = computeOpenCodeSegments(items);
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
    const items = [assistant(T0), tool(T0 + 1000)];
    const segments = computeOpenCodeSegments(items);
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
    const items = [user(T0), assistant(T0 + 1000), user(T0 + 2000)];
    const segments = computeOpenCodeSegments(items);
    // [t1 user, t1 assistant, t2 user] — t2 has no body
    expect(segments).toHaveLength(3);
    expect(segments[2]).toMatchObject({ speaker: 'user', turnIndex: 2, startIndex: 2, messageCount: 1 });
  });

  it('CORRECTNESS TRAP: durations use numeric epoch-ms arithmetic, not new Date()', () => {
    // Large epoch-ms values that would parse to NaN if passed through
    // `new Date(string)`. Plain subtraction must yield the exact delta.
    const start = 1_700_000_000_000;
    const items = [user(start), assistant(start + 12_345)];
    const segments = computeOpenCodeSegments(items);
    const assistantSeg = segments.find((s) => s.speaker === 'assistant');
    expect(assistantSeg?.durationMs).toBe(12_345);
    expect(Number.isNaN(assistantSeg?.durationMs)).toBe(false);
  });
});
