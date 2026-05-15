// Spec tests for the Codex timeline bar (CF-379).
//
// Locks the contract on segmentation (2N segments for N completed turns,
// alternating user/assistant), Claude-style tooltip copy ("User: <dur>, 1
// item" / "Codex: <dur>, N items" with no TTFT or turn index), and
// click-to-seek targets.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import CodexTimelineBar from './CodexTimelineBar';
import type { CodexRenderItem } from '@/types/codexRenderItem';

function user(timestamp: string, text = 'hi'): CodexRenderItem {
  return { kind: 'user', lineId: '0', timestamp, text };
}

function assistant(timestamp: string): CodexRenderItem {
  return { kind: 'assistant', lineId: '0', timestamp, text: 'hello', phase: 'final', model: 'gpt-5' };
}

function turnSep(
  timestamp: string,
  turnIndex: number,
  durationMs: number,
  timeToFirstTokenMs?: number,
): CodexRenderItem {
  return { kind: 'turn_separator', lineId: '0', timestamp, turnIndex, durationMs, timeToFirstTokenMs };
}

const twoTurnSession: CodexRenderItem[] = [
  user('2026-05-13T18:00:00Z'),
  assistant('2026-05-13T18:00:05Z'),
  turnSep('2026-05-13T18:00:06Z', 1, 6000, 1200),
  user('2026-05-13T18:01:00Z'), // 54s gap
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

  it('renders 2N segments for N completed turns (user + assistant alternating)', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    expect(segments).toHaveLength(4);
  });

  it('click on a user segment seeks to the user item index', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={onSeek} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    // Order: [turn1 user, turn1 assistant, turn2 user, turn2 assistant]
    fireEvent.click(segments[2]!); // turn 2 user
    expect(onSeek).toHaveBeenCalledWith(3); // index of turn 2's user item
  });

  it('click on an assistant segment seeks to the first item after the user', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={onSeek} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.click(segments[1]!); // turn 1 assistant
    expect(onSeek).toHaveBeenCalledWith(1); // first non-user item in turn 1
  });

  it('hover on a user segment shows "User: <dur>, 1 item"', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[2]!); // turn 2 user (real 54s gap)
    expect(screen.getByText(/^User:\s+54s,\s+1\s+item$/)).toBeInTheDocument();
  });

  it('hover on an assistant segment shows "Codex: <dur>, N items"', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[1]!); // turn 1 assistant: 6s, 2 items
    expect(screen.getByText(/^Codex:\s+6s,\s+2\s+items$/)).toBeInTheDocument();
  });

  it('tooltips do not show TTFT', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[1]!);
    expect(screen.queryByText(/TTFT/i)).toBeNull();
  });

  it('tooltips do not show turn index', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    fireEvent.mouseEnter(segments[1]!);
    expect(screen.queryByText(/Turn\s+\d+/i)).toBeNull();
  });

  it('renders 2 segments for an in-flight turn (user + assistant durationMs=0)', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      assistant('2026-05-13T18:00:05Z'),
    ];
    const { container } = render(
      <CodexTimelineBar items={items} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    expect(segments).toHaveLength(2);
  });

  it('renders 1 segment for a user-only turn', () => {
    const items: CodexRenderItem[] = [
      user('2026-05-13T18:00:00Z'),
      turnSep('2026-05-13T18:00:01Z', 1, 1000),
    ];
    const { container } = render(
      <CodexTimelineBar items={items} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll('[data-codex-segment]');
    expect(segments).toHaveLength(1);
    expect(segments[0]).toHaveAttribute('data-turn-index', '1');
  });

  it('applies separate CSS classes to user vs assistant segments', () => {
    const { container } = render(
      <CodexTimelineBar items={twoTurnSession} selectedIndex={0} onSeek={() => undefined} />,
    );
    const segments = container.querySelectorAll<HTMLElement>('[data-codex-segment]');
    // turn1 user, turn1 assistant should have different class lists
    expect(segments[0]!.className).not.toEqual(segments[1]!.className);
    // Same speaker → same class list
    expect(segments[0]!.className).toEqual(segments[2]!.className);
    expect(segments[1]!.className).toEqual(segments[3]!.className);
  });
});
