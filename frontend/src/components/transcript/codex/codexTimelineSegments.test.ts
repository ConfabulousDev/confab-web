// Spec tests for the Codex timeline-segment computation.
//
// Locks the contract: one segment per turn boundary (CodexTurnSeparatorItem),
// trailing in-flight segment for items after the last separator, and
// timing/index metadata preserved from the separator.

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

function turnSep(
  timestamp: string,
  turnIndex: number,
  durationMs: number,
  timeToFirstTokenMs?: number,
): CodexRenderItem {
  return { kind: 'turn_separator', timestamp, turnIndex, durationMs, timeToFirstTokenMs };
}

describe('computeCodexSegments', () => {
  it('returns an empty array for empty items', () => {
    expect(computeCodexSegments([])).toEqual([]);
  });

  it('returns one trailing segment for items without a turn separator (in-flight)', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(1);
    expect(segments[0]?.startIndex).toBe(0);
    expect(segments[0]?.endIndex).toBe(1);
    expect(segments[0]?.messageCount).toBe(2);
  });

  it('slices on each turn_separator and returns N segments for N separators', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
      turnSep('2026-05-13T18:00:06Z', 1, 6000, 1200),
      user('2026-05-13T18:01:00Z'),
      assistant('2026-05-13T18:01:03Z'),
      turnSep('2026-05-13T18:01:04Z', 2, 4000, 800),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(2);
    expect(segments[0]?.startIndex).toBe(0);
    expect(segments[0]?.endIndex).toBe(2);
    expect(segments[1]?.startIndex).toBe(3);
    expect(segments[1]?.endIndex).toBe(5);
  });

  it('includes a trailing in-flight segment for items after the last separator', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
      turnSep('2026-05-13T18:00:06Z', 1, 6000, 1200),
      user('2026-05-13T18:01:00Z'),
      toolCall('2026-05-13T18:01:01Z'),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(2);
    expect(segments[1]?.startIndex).toBe(3);
    expect(segments[1]?.endIndex).toBe(4);
  });

  it('preserves turnIndex from the separator on each completed segment', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
      user('2026-05-13T18:00:10Z'),
      turnSep('2026-05-13T18:00:11Z', 2, 1000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments[0]?.turnIndex).toBe(1);
    expect(segments[1]?.turnIndex).toBe(2);
  });

  it('uses durationMs from the separator', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 12345, 678),
    ];
    const segments = computeCodexSegments(items);
    expect(segments[0]?.durationMs).toBe(12345);
    expect(segments[0]?.timeToFirstTokenMs).toBe(678);
  });

  it('omits timeToFirstTokenMs when the separator did not carry one', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments[0]?.timeToFirstTokenMs).toBeUndefined();
  });

  it('messageCount equals endIndex - startIndex + 1 (counts every render item in the turn)', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      { kind: 'reasoning_hidden', timestamp: '2026-05-13T18:00:01Z' },
      assistant('2026-05-13T18:00:02Z'),
      toolCall('2026-05-13T18:00:03Z'),
      assistant('2026-05-13T18:00:04Z'),
      turnSep('2026-05-13T18:00:05Z', 1, 5000),
    ];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(1);
    // 6 items total (user, reasoning_hidden, assistant, tool_call, assistant, turn_separator)
    expect(segments[0]?.messageCount).toBe(6);
  });

  it('handles a single trailing item with no separator', () => {
    const items: CodexRenderItem[] = [user('2026-05-13T18:00:00Z')];
    const segments = computeCodexSegments(items);
    expect(segments).toHaveLength(1);
    expect(segments[0]?.messageCount).toBe(1);
  });

  it('handles back-to-back turn_separators (degenerate but valid) as N+1 segments only when the second has content after it', () => {
    // Two separators with content only between them: 2 segments.
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
      turnSep('2026-05-13T18:00:02Z', 2, 1000),
    ];
    const segments = computeCodexSegments(items);
    // Two separators → two completed segments; second segment is just the separator itself.
    expect(segments).toHaveLength(2);
    expect(segments[1]?.startIndex).toBe(2);
    expect(segments[1]?.endIndex).toBe(2);
    expect(segments[1]?.messageCount).toBe(1);
  });
});
