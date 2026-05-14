// Spec tests for the Codex timeline bar. Locks the contract on what the bar
// renders, what the tooltip says, and how click-to-seek calls back.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import CodexTimelineBar from './CodexTimelineBar';
import type { CodexRenderItem } from '@/types/codexRenderItem';

function user(timestamp: string, text = 'hi'): CodexRenderItem {
  return { kind: 'user', timestamp, text };
}

function assistant(timestamp: string): CodexRenderItem {
  return { kind: 'assistant', timestamp, text: 'hello', phase: 'final', model: 'gpt-5' };
}

function turnSep(
  timestamp: string,
  turnIndex: number,
  durationMs: number,
  timeToFirstTokenMs?: number,
): CodexRenderItem {
  return { kind: 'turn_separator', timestamp, turnIndex, durationMs, timeToFirstTokenMs };
}

const twoTurnSession: CodexRenderItem[] = [
  user('2026-05-13T18:00:00Z'),
  assistant('2026-05-13T18:00:05Z'),
  turnSep('2026-05-13T18:00:06Z', 1, 6000, 1200),
  user('2026-05-13T18:01:00Z'),
  assistant('2026-05-13T18:01:03Z'),
  turnSep('2026-05-13T18:01:04Z', 2, 4000, 800),
];

describe('CodexTimelineBar', () => {
  it('renders nothing when there are no items', () => {
    const { container } = render(
      <CodexTimelineBar items={[]} selectedIndex={0} onSeek={() => undefined} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders one clickable segment per turn', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={onSeek} />,
    );
    // Two segments → two clickable children of the segments container.
    // We assert via a count of any element marked as a segment.
    const segments = container.querySelectorAll('[data-codex-segment]');
    expect(segments).toHaveLength(2);
  });

  it('click on a segment calls onSeek with that segment startIndex', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={onSeek} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    // Click the second segment → its startIndex is 3 (after first separator).
    fireEvent.click(segments[1]!);
    expect(onSeek).toHaveBeenCalledWith(3);
  });

  it('hover tooltip shows turn label, duration, TTFT, and item count', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[0]!);

    // First turn: 6s duration, TTFT 1.2s, 3 items (user + assistant + separator).
    expect(screen.getByText(/Turn\s+1/i)).toBeInTheDocument();
    expect(screen.getByText(/6s/)).toBeInTheDocument();
    expect(screen.getByText(/TTFT\s+1\.2s|TTFT\s+1200ms|TTFT\s+1s/)).toBeInTheDocument();
    expect(screen.getByText(/3\s+items/)).toBeInTheDocument();
  });

  it('hover tooltip omits TTFT when the separator did not carry one', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
      turnSep('2026-05-13T18:00:06Z', 1, 6000), // no TTFT
    ];
    const { container } = render(
      <CodexTimelineBar items={items} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[0]!);

    expect(screen.queryByText(/TTFT/i)).toBeNull();
  });

  it('renders a single segment for an in-flight session (no separators)', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
    ];
    const { container } = render(
      <CodexTimelineBar items={items} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    expect(segments).toHaveLength(1);
  });

  it('uses singular "item" wording for a 1-item segment', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
      assistant('2026-05-13T18:00:02Z'),
      // No closing separator → second segment has 1 item (just the assistant).
    ];
    const { container } = render(
      <CodexTimelineBar items={items} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[1]!);
    expect(screen.getByText(/1\s+item(?!s)/i)).toBeInTheDocument();
  });
});
