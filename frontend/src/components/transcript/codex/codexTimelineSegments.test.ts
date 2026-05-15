// Spec tests for the Codex timeline-segment computation.
//
// Locks the contract (per CF-379): each turn emits up to two segments — a
// user thinking-gap segment (synthetic 1s for turn 1, real wall-clock gap
// from the prior separator for later turns, clamped to 1s minimum) and an
// assistant body segment. Slices with no user item collapse to a single
// assistant segment.

import { describe, it, expect } from 'vitest';
import { computeCodexSegments } from './codexTimelineSegments';
import type { CodexRenderItem } from '@/types/codexRenderItem';

function user(timestamp: string, text = 'hi'): CodexRenderItem {
  return { kind: 'user', timestamp, text };
}

function assistant(timestamp: string, text = 'hello'): CodexRenderItem {
  return { kind: 'assistant', timestamp, text, phase: 'final', model: 'gpt-5' };
}

function toolCall(timestamp: string, callId = 'c1'): CodexRenderItem {
  return {
    kind: 'tool_call',
    timestamp,
    toolName: 'exec_command',
    callId,
    rawInput: { cmd: 'pwd' },
    status: 'completed',
  };
}

function compacted(timestamp: string, replacementCount = 5): CodexRenderItem {
  return { kind: 'compacted', timestamp, replacementCount };
}

function turnSep(
  timestamp: string,
  turnIndex: number,
  durationMs: number,
): CodexRenderItem {
  return { kind: 'turn_separator', timestamp, turnIndex, durationMs };
}

describe('computeCodexSegments', () => {
  it('returns an empty array for empty items', () => {
    expect(computeCodexSegments([])).toEqual([]);
  });

  it('renders user + assistant segments for a single completed turn with synthetic 1s user gap', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
      turnSep('2026-05-13T18:00:06Z', 1, 6000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toMatchObject({
      speaker: 'user',
      turnIndex: 1,
      durationMs: 1000,
      startIndex: 0,
      endIndex: 0,
      messageCount: 1,
    });
    expect(segments[1]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      durationMs: 6000,
      startIndex: 1,
      endIndex: 2,
      messageCount: 2,
    });
  });

  it('uses the wall-clock gap from the previous separator as turn 2 user duration', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
      turnSep('2026-05-13T18:00:06Z', 1, 6000),
      user('2026-05-13T18:01:36Z'), // 90s after the separator
      assistant('2026-05-13T18:01:40Z'),
      turnSep('2026-05-13T18:01:41Z', 2, 5000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(4);
    expect(segments[2]).toMatchObject({
      speaker: 'user',
      turnIndex: 2,
      durationMs: 90_000,
      startIndex: 3,
      endIndex: 3,
      messageCount: 1,
    });
    expect(segments[3]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 2,
      durationMs: 5000,
      startIndex: 4,
      endIndex: 5,
      messageCount: 2,
    });
  });

  it('emits only a user segment for a user-only turn', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      speaker: 'user',
      turnIndex: 1,
      messageCount: 1,
    });
  });

  it('emits only an assistant segment when a slice has no user item (compaction case)', () => {
    const items: CodexRenderItem[] = [
      compacted('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      startIndex: 0,
      endIndex: 1,
      messageCount: 2,
    });
  });

  it('renders user + assistant segments for an in-flight turn with durationMs=0 on assistant', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toMatchObject({
      speaker: 'user',
      turnIndex: 1,
      durationMs: 1000, // synthetic — no prior separator
      messageCount: 1,
    });
    expect(segments[1]).toMatchObject({
      speaker: 'assistant',
      turnIndex: 1,
      durationMs: 0,
      messageCount: 1,
    });
  });

  it('renders a single assistant segment for an in-flight slice with no user item', () => {
    const items: CodexRenderItem[] = [
      toolCall('2026-05-13T18:00:00Z'),
      toolCall('2026-05-13T18:00:01Z', 'c2'),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(1);
    expect(segments[0]?.speaker).toBe('assistant');
    expect(segments[0]?.durationMs).toBe(0);
  });

  it('clamps zero / negative computed user durations to 1s', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
      // Second user message at the *same* instant as the separator → 0ms gap
      user('2026-05-13T18:00:01Z'),
      turnSep('2026-05-13T18:00:02Z', 2, 1000),
    ];
    const segments = computeCodexSegments(items);
    // Turn 2's user segment is the third segment (after turn 1 user + turn 1 separator-only segment)
    const turn2User = segments.find((s) => s.turnIndex === 2 && s.speaker === 'user');
    expect(turn2User?.durationMs).toBe(1000);
  });

  it('preserves turnIndex on both segments of a turn', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:01Z'),
      turnSep('2026-05-13T18:00:02Z', 7, 2000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments.every((s) => s.turnIndex === 7)).toBe(true);
  });

  it('messageCount on user is always 1; assistant counts the rest of the slice', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      { kind: 'reasoning_hidden', timestamp: '2026-05-13T18:00:01Z' },
      assistant('2026-05-13T18:00:02Z'),
      toolCall('2026-05-13T18:00:03Z'),
      assistant('2026-05-13T18:00:04Z'),
      turnSep('2026-05-13T18:00:05Z', 1, 5000),
    ];
    const segments = computeCodexSegments(items);
    const userSeg = segments.find((s) => s.speaker === 'user');
    const assistantSeg = segments.find((s) => s.speaker === 'assistant');
    expect(userSeg?.messageCount).toBe(1);
    // 5 items after user: reasoning_hidden, assistant, tool_call, assistant, separator
    expect(assistantSeg?.messageCount).toBe(5);
  });

  it('does not carry timeToFirstTokenMs on segments (field removed from shape)', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:01Z'),
      // Separator with a TTFT — must be ignored when shaping the segment
      { kind: 'turn_separator', timestamp: '2026-05-13T18:00:02Z', turnIndex: 1, durationMs: 2000, timeToFirstTokenMs: 500 },
    ];
    const segments = computeCodexSegments(items);
    for (const seg of segments) {
      expect('timeToFirstTokenMs' in seg).toBe(false);
    }
  });
});
