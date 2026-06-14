// Tests for the OpenCode transcript pane's minimap / timeline bar wiring
// (ag2x). Covers: bar present only when segments exist; click-to-seek scrolls
// the virtualizer; the scroll listener updates first-visible; deep-link scroll
// still works after the `.container` refactor.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import OpenCodeTranscriptPane from './OpenCodeTranscriptPane';
import type { OpenCodeRenderItem } from './opencodeCategories';
import type { TokenUsage } from '@/utils/tokenStats';

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

// ---------------------------------------------------------------------------
// hfk7 — cost side-rail (shared CostBar) wiring.
//
// The virtualizer mock above lays out every row, so we CAN assert both the
// per-row badge text and the rail. The CostBar root carries a recognisable
// `title="Color intensity..."` attribute and renders null when totalCost === 0.
// Badge + rail are routed through opencodeAdapter.calculateMessageCost, which
// prefers a reported `info.cost` and otherwise falls back to the pricing table
// (the frozen fixture installed globally in src/test/setup.ts).
// ---------------------------------------------------------------------------

const COST_RAIL = '[title*="Color intensity"]';

// gemini-2.5-pro fixture price: input 1.25/M, output 5.0/M.
// 1_000_000 in + 100_000 out → 1.25 + 0.5 = $1.75 (clean for assertions).
const FALLBACK_USAGE: TokenUsage = {
  input: 1_000_000,
  output: 100_000,
  cacheWrite: 0,
  cacheWrite1h: 0,
  cacheRead: 0,
};

function assistantWithCost(
  id: string,
  timeCreated: number,
  cost: number,
): OpenCodeRenderItem {
  return { kind: 'assistant', id, text: 'hello', model: 'gemini-2.5-pro', cost, timeCreated };
}

function assistantFallback(id: string, timeCreated: number): OpenCodeRenderItem {
  // No reported `cost` → adapter prices via the pricing table.
  return {
    kind: 'assistant',
    id,
    text: 'hello',
    model: 'gemini-2.5-pro',
    usage: FALLBACK_USAGE,
    timeCreated,
  };
}

describe('OpenCodeTranscriptPane cost rail', () => {
  it('renders the CostBar with a positive total when cost mode is on', () => {
    const items: OpenCodeRenderItem[] = [
      user('u1', T0),
      assistantWithCost('a1', T0 + 5000, 0.5),
    ];
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        isCostMode
      />,
    );
    expect(container.querySelector(COST_RAIL)).not.toBeNull();
  });

  it('does NOT render the CostBar when cost mode is off', () => {
    const items: OpenCodeRenderItem[] = [
      user('u1', T0),
      assistantWithCost('a1', T0 + 5000, 0.5),
    ];
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        isCostMode={false}
      />,
    );
    expect(container.querySelector(COST_RAIL)).toBeNull();
  });

  it('does NOT render the CostBar when cost mode is on but total cost is zero', () => {
    // Assistant with neither reported cost nor usage → calculateMessageCost = 0.
    const items: OpenCodeRenderItem[] = [user('u1', T0), assistant('a1', T0 + 5000)];
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        isCostMode
      />,
    );
    expect(container.querySelector(COST_RAIL)).toBeNull();
  });

  it('a filtered-out assistant row still contributes to the rail total', () => {
    // costByIndex is keyed by UNFILTERED index; a filtered-out assistant must
    // still count toward the rail. Drop a1 from the filtered list, but it has
    // the only cost — the rail must still render (totalCost > 0).
    const items: OpenCodeRenderItem[] = [
      user('u1', T0),
      assistantWithCost('a1', T0 + 5000, 0.42),
      user('u2', T0 + 95_000),
    ];
    const filtered = items.filter((it) => it.id !== 'a1');
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={filtered}
        loading={false}
        error={null}
        isCostMode
      />,
    );
    // Rail renders because the filtered-out row's cost is still summed.
    expect(container.querySelector(COST_RAIL)).not.toBeNull();
    // The filtered-out row's badge is absent (row not in the list).
    expect(container.textContent).not.toContain('$0.42');
  });

  it('badge uses the reported info.cost, matching the rail source of truth', () => {
    const items: OpenCodeRenderItem[] = [
      user('u1', T0),
      assistantWithCost('a1', T0 + 5000, 0.5),
    ];
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        isCostMode
      />,
    );
    // formatCost(0.5) → '$0.50'.
    expect(container.textContent).toContain('$0.50');
  });

  it('badge uses the pricing fallback when there is no reported cost', () => {
    const items: OpenCodeRenderItem[] = [user('u1', T0), assistantFallback('a1', T0 + 5000)];
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        isCostMode
      />,
    );
    // calculateCost('opencode','gemini-2.5-pro',FALLBACK_USAGE) → $1.75.
    expect(container.querySelector(COST_RAIL)).not.toBeNull();
    expect(container.textContent).toContain('$1.75');
  });

  it('clicking a rail segment scrolls the virtualizer to the right row', () => {
    const items: OpenCodeRenderItem[] = [
      user('u1', T0),
      assistantWithCost('a1', T0 + 5000, 0.5),
      user('u2', T0 + 95_000),
      assistantWithCost('a2', T0 + 100_000, 0.5),
    ];
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        isCostMode
      />,
    );
    // The CostBar's clickable cells live inside its segments container; click
    // a later one and confirm the virtualizer seeks to a non-zero row.
    const rail = container.querySelector(COST_RAIL)!;
    const cells = rail.querySelectorAll<HTMLElement>('div');
    // The first child div is the segments container; its children are segments.
    const segContainer = cells[0]!;
    const segs = segContainer.children;
    expect(segs.length).toBeGreaterThan(1);
    fireEvent.click(segs[segs.length - 1]!);
    // Last segment → unfiltered index 3 → filtered index 3.
    expect(scrollToIndex).toHaveBeenCalledWith(3, expect.objectContaining({ align: 'start' }));
  });
});
