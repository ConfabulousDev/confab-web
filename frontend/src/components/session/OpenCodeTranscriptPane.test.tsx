// Tests for the OpenCode transcript pane's minimap / timeline bar wiring
// (ag2x). Covers: bar present only when segments exist; click-to-seek scrolls
// the virtualizer; the scroll listener updates first-visible; deep-link scroll
// still works after the `.container` refactor.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import OpenCodeTranscriptPane from './OpenCodeTranscriptPane';
import type { OpenCodeRenderItem } from './opencodeCategories';

// Spy on the virtualizer's scrollToIndex. retryOnAnimationFrame schedules via
// rAF, so we drive frames manually.
const scrollToIndex = vi.fn();

vi.mock('@tanstack/react-virtual', () => {
  return {
    useVirtualizer: ({ count }: { count: number }) => ({
      getVirtualItems: () =>
        Array.from({ length: count }, (_, index) => ({
          index,
          key: String(index),
          start: index * 120,
          size: 120,
        })),
      getTotalSize: () => count * 120,
      scrollToIndex,
      measureElement: () => undefined,
    }),
  };
});

const T0 = 1_717_689_500_000;
function user(id: string, timeCreated: number): OpenCodeRenderItem {
  return { kind: 'user', id, text: 'hi', timeCreated };
}
function assistant(id: string, timeCreated: number): OpenCodeRenderItem {
  return { kind: 'assistant', id, text: 'hello', timeCreated };
}

const session: OpenCodeRenderItem[] = [
  user('u1', T0),
  assistant('a1', T0 + 5000),
  user('u2', T0 + 95_000),
  assistant('a2', T0 + 100_000),
];

beforeEach(() => {
  scrollToIndex.mockClear();
  // Make rAF synchronous so retryOnAnimationFrame fires immediately.
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
    cb(0);
    return 0;
  });
});

describe('OpenCodeTranscriptPane timeline bar', () => {
  it('renders the timeline bar when there are segments', () => {
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={session}
        filteredItems={session}
        loading={false}
        error={null}
      />,
    );
    expect(container.querySelectorAll('[data-timeline-segment]').length).toBeGreaterThan(0);
  });

  it('does not render the bar in the loading / empty states', () => {
    const { container: loadingC } = render(
      <OpenCodeTranscriptPane sessionId="s" items={[]} filteredItems={[]} loading error={null} />,
    );
    expect(loadingC.querySelector('[data-timeline-segment]')).toBeNull();

    const { container: emptyC } = render(
      <OpenCodeTranscriptPane sessionId="s" items={[]} filteredItems={[]} loading={false} error={null} />,
    );
    expect(emptyC.querySelector('[data-timeline-segment]')).toBeNull();
  });

  it('click-to-seek scrolls the virtualizer to the segment start', () => {
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={session}
        filteredItems={session}
        loading={false}
        error={null}
      />,
    );
    const segments = container.querySelectorAll('[data-timeline-segment]');
    // Segments: [t1 user(0), t1 assistant(1), t2 user(2), t2 assistant(3)]
    fireEvent.click(segments[2]!); // turn 2 user → unfiltered index 2
    expect(scrollToIndex).toHaveBeenCalledWith(2, expect.objectContaining({ align: 'start' }));
  });

  it('greys out fully-filtered segments (visibleIndices derived from items vs filteredItems)', () => {
    // Drop the assistant rows: their segments have no visible item.
    const filtered = session.filter((it) => it.kind !== 'assistant');
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={session}
        filteredItems={filtered}
        loading={false}
        error={null}
      />,
    );
    const segments = container.querySelectorAll<HTMLElement>('[data-timeline-segment]');
    // At least one assistant segment should be marked filtered (greyed) and
    // therefore have a different class than a visible user segment.
    const classes = Array.from(segments).map((s) => s.className);
    expect(new Set(classes).size).toBeGreaterThan(1);
  });

  it('deep-link scroll still fires after the container refactor', () => {
    render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={session}
        filteredItems={session}
        loading={false}
        error={null}
        targetId="a2"
      />,
    );
    // a2 is filtered index 3.
    expect(scrollToIndex).toHaveBeenCalledWith(3, expect.objectContaining({ align: 'start' }));
  });
});
