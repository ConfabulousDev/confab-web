// Tests for the OpenCode transcript pane's minimap / timeline bar wiring
// (ag2x), cost rail (hfk7), and Cmd-F in-transcript search (5p9j). Covers: bar
// present only when segments exist; click-to-seek scrolls the virtualizer; the
// scroll listener updates first-visible; deep-link scroll still works after the
// `.container` refactor; search opens via Cmd-F, navigates matches, scrolls to
// matches in unmounted rows, highlights, and force-opens collapsed <details>.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, act } from '@testing-library/react';
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

// ---------------------------------------------------------------------------
// 5p9j — Cmd-F in-transcript search (shared useTranscriptSearch toolkit).
//
// The search query is debounced (150ms) and the highlight query (300ms), so
// these tests run on fake timers and advance them after typing. The
// virtualizer mock above lays out every row, so we assert on the rendered
// <mark>s and the match-count text directly, and spy on scrollToIndex for the
// scroll-to-(unmounted)-match behavior.
// ---------------------------------------------------------------------------

function assistantWithReasoning(id: string, timeCreated: number): OpenCodeRenderItem {
  return {
    kind: 'assistant',
    id,
    text: 'visible body text',
    reasoning: 'hidden zebra reasoning',
    timeCreated,
  };
}

function toolItem(id: string, timeCreated: number): OpenCodeRenderItem {
  return {
    kind: 'tool',
    id,
    toolName: 'Bash',
    status: 'completed',
    input: 'ls -la',
    output: 'collapsed quokka output',
    timeCreated,
  };
}

// Open the search bar, type a query, and flush the debounce timers.
function openAndType(container: HTMLElement, query: string) {
  act(() => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', metaKey: true }));
  });
  const input = container.querySelector<HTMLInputElement>('input[aria-label="Search transcript"]');
  expect(input).not.toBeNull();
  act(() => {
    fireEvent.change(input!, { target: { value: query } });
  });
  act(() => {
    vi.advanceTimersByTime(400);
  });
  return input!;
}

describe('OpenCodeTranscriptPane search (5p9j)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // rAF synchronous so retryOnAnimationFrame / the scroll-to-mark dance fire
    // without real frames (overrides the file-level rAF stub, which is fine).
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 0;
    });
    // jsdom doesn't implement scrollIntoView; the scroll-to-mark dance calls it.
    Element.prototype.scrollIntoView = vi.fn();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('Cmd-F opens the search bar', () => {
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={session} filteredItems={session} loading={false} error={null} />,
    );
    expect(container.querySelector('input[aria-label="Search transcript"]')).toBeNull();
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', metaKey: true }));
    });
    expect(container.querySelector('input[aria-label="Search transcript"]')).not.toBeNull();
  });

  it('typing a query shows the "N of M" count and renders <mark>s', () => {
    const items: OpenCodeRenderItem[] = [
      { kind: 'user', id: 'u1', text: 'needle here', timeCreated: T0 },
      { kind: 'assistant', id: 'a1', text: 'no match', timeCreated: T0 + 1000 },
      { kind: 'user', id: 'u2', text: 'another needle row', timeCreated: T0 + 2000 },
    ];
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={items} loading={false} error={null} />,
    );
    openAndType(container, 'needle');
    // 2 matching rows.
    expect(container.textContent).toContain('1 of 2');
    expect(container.querySelectorAll('mark').length).toBeGreaterThan(0);
    // The active match's <mark> carries the active class.
    expect(container.querySelector('mark.search-highlight-active')).not.toBeNull();
  });

  it('Enter / Shift-Enter navigate matches', () => {
    const items: OpenCodeRenderItem[] = [
      { kind: 'user', id: 'u1', text: 'needle one', timeCreated: T0 },
      { kind: 'user', id: 'u2', text: 'needle two', timeCreated: T0 + 1000 },
      { kind: 'user', id: 'u3', text: 'needle three', timeCreated: T0 + 2000 },
    ];
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={items} loading={false} error={null} />,
    );
    const input = openAndType(container, 'needle');
    expect(container.textContent).toContain('1 of 3');
    act(() => { fireEvent.keyDown(input, { key: 'Enter' }); });
    expect(container.textContent).toContain('2 of 3');
    act(() => { fireEvent.keyDown(input, { key: 'Enter', shiftKey: true }); });
    expect(container.textContent).toContain('1 of 3');
  });

  it('Escape closes the search bar', () => {
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={session} filteredItems={session} loading={false} error={null} />,
    );
    const input = openAndType(container, 'hi');
    expect(container.querySelector('input[aria-label="Search transcript"]')).not.toBeNull();
    act(() => { fireEvent.keyDown(input, { key: 'Escape' }); });
    expect(container.querySelector('input[aria-label="Search transcript"]')).toBeNull();
  });

  it('a match in a far-down (would-be unmounted) row triggers scrollToIndex with that index', () => {
    // 30 rows; only the last contains the needle.
    const items: OpenCodeRenderItem[] = Array.from({ length: 30 }, (_, i) => ({
      kind: 'user',
      id: `u${i}`,
      text: i === 29 ? 'rare beacon term' : `filler ${i}`,
      timeCreated: T0 + i * 1000,
    }));
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={items} loading={false} error={null} />,
    );
    openAndType(container, 'beacon');
    // The current-match effect scrolls the virtualizer to the match's filtered
    // index (== virtual index for OpenCode), centering it.
    expect(scrollToIndex).toHaveBeenCalledWith(29, expect.objectContaining({ align: 'center' }));
  });

  it('changing the filter recomputes matches and resets the active match', () => {
    const items: OpenCodeRenderItem[] = [
      { kind: 'user', id: 'u1', text: 'needle alpha', timeCreated: T0 },
      { kind: 'tool', id: 't1', toolName: 'Bash', status: 'ok', input: 'needle beta', timeCreated: T0 + 1000 },
      { kind: 'user', id: 'u2', text: 'needle gamma', timeCreated: T0 + 2000 },
    ];
    const { container, rerender } = render(
      <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={items} loading={false} error={null} />,
    );
    const input = openAndType(container, 'needle');
    expect(container.textContent).toContain('1 of 3');
    // Advance to match 2.
    act(() => { fireEvent.keyDown(input, { key: 'Enter' }); });
    expect(container.textContent).toContain('2 of 3');
    // Filter out the tool row → matches recompute (3 → 2) and active resets to 1.
    const filtered = items.filter((it) => it.kind !== 'tool');
    act(() => {
      rerender(
        <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={filtered} loading={false} error={null} />,
      );
      vi.advanceTimersByTime(400);
    });
    expect(container.textContent).toContain('1 of 2');
  });

  it('decision 5: a match inside a reasoning <details> forces it open', () => {
    const items: OpenCodeRenderItem[] = [assistantWithReasoning('a1', T0)];
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={items} loading={false} error={null} />,
    );
    // Before search: reasoning <details> is closed.
    const detailsBefore = container.querySelector('details');
    expect(detailsBefore).not.toBeNull();
    expect(detailsBefore!.open).toBe(false);
    // Search for a term only present inside the collapsed reasoning.
    openAndType(container, 'zebra');
    const details = container.querySelector('details');
    expect(details).not.toBeNull();
    expect(details!.open).toBe(true);
    // And the match is highlighted inside it.
    expect(details!.querySelector('mark')).not.toBeNull();
  });

  it('decision 5: a match inside a tool-output <details> forces it open', () => {
    const items: OpenCodeRenderItem[] = [toolItem('t1', T0)];
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={items} filteredItems={items} loading={false} error={null} />,
    );
    const detailsBefore = container.querySelector('details');
    expect(detailsBefore!.open).toBe(false);
    openAndType(container, 'quokka');
    const details = container.querySelector('details');
    expect(details!.open).toBe(true);
    expect(details!.querySelector('mark')).not.toBeNull();
  });
});

// ---------------------------------------------------------------------------
// 6h7m — day/idle-gap divider virtualizer-wiring regression coverage.
//
// OpenCode previously had NO separator/divider rows at all, so the filtered
// index WAS the virtual index everywhere. Once a divider can be injected,
// those two indices diverge whenever a divider precedes the target row —
// this locks down that the pane routes scrollToIndex through the new
// filteredIndex -> virtualIndex map instead of the raw filtered index.
// ---------------------------------------------------------------------------

describe('OpenCodeTranscriptPane day-boundary divider wiring (6h7m)', () => {
  const may13_2359 = new Date(2026, 4, 13, 23, 59, 0).getTime();
  const may14_0001 = new Date(2026, 4, 14, 0, 1, 0).getTime();
  const dayBoundaryItems: OpenCodeRenderItem[] = [
    user('u1', may13_2359),
    assistant('a1', may14_0001),
  ];

  it('renders a divider row between items on different calendar days', () => {
    const { container } = render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={dayBoundaryItems}
        filteredItems={dayBoundaryItems}
        loading={false}
        error={null}
      />,
    );
    expect(container.textContent).toMatch(/\w+day, May/);
  });

  it('deep-link scroll lands on the target row, not the divider, when a separator precedes it', () => {
    render(
      <OpenCodeTranscriptPane
        sessionId="s"
        items={dayBoundaryItems}
        filteredItems={dayBoundaryItems}
        loading={false}
        error={null}
        targetId="a1"
      />,
    );
    // Virtual layout: [item(u1)=0, separator=1, item(a1)=2]. Without the
    // filteredIndex->virtualIndex fix this would incorrectly fire with 1
    // (a1's filtered index), landing on the divider instead of the row.
    expect(scrollToIndex).toHaveBeenCalledWith(2, expect.objectContaining({ align: 'start' }));
  });

  it('does not inject a divider for the default same-session fixture (all within seconds)', () => {
    const { container } = render(
      <OpenCodeTranscriptPane sessionId="s" items={session} filteredItems={session} loading={false} error={null} />,
    );
    expect(container.textContent).not.toMatch(/\w+day, /);
  });
});
