// 6h7m: virtualizer-wiring regression coverage for the new day/idle-gap
// divider layer. Cursor previously had NO separator/divider rows at all, so
// the filtered index WAS the virtual index everywhere (deep-link scroll,
// search-match scroll, skip-nav). Once dividers can be injected, those two
// indices diverge whenever a divider precedes the target row — this file
// locks down that the pane now routes scrollToIndex calls through the new
// filteredIndex -> virtualIndex map instead of using the raw filtered index.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import CursorTranscriptPane from './CursorTranscriptPane';
import type { CursorRenderItem } from './cursorCategories';

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

beforeEach(() => {
  scrollToIndex.mockClear();
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
    cb(0);
    return 0;
  });
});

// ids follow the real Cursor render-item convention: numeric line index
// (attachCursorTimestamps's `lineIndexOf` parses the id as an integer).
const items: CursorRenderItem[] = [
  { kind: 'user', id: '0', text: 'first' },
  { kind: 'assistant', id: '1', text: 'second' },
];

// firstSeen/lastSyncAt a day apart so attachCursorTimestamps interpolates the
// 2 items onto different calendar days, forcing a day-boundary divider
// between them (the second item is preceded by a separator).
const firstSeen = new Date(2026, 4, 13, 0, 0, 0).toISOString();
const lastSyncAt = new Date(2026, 4, 14, 0, 0, 0).toISOString();

describe('CursorTranscriptPane day-boundary divider wiring (6h7m)', () => {
  it('renders a divider row between items on different estimated calendar days', () => {
    const { container } = render(
      <CursorTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        firstSeen={firstSeen}
        lastSyncAt={lastSyncAt}
      />,
    );
    // A weekday name (e.g. "Thursday") only appears in the day-boundary label.
    expect(container.textContent).toMatch(/\w+day, May/);
  });

  it('deep-link scroll lands on the target row, not the divider, when a separator precedes it', () => {
    render(
      <CursorTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
        firstSeen={firstSeen}
        lastSyncAt={lastSyncAt}
        targetId="1"
      />,
    );
    // Virtual layout: [item(id=0)=0, separator=1, item(id=1)=2]. Without the
    // filteredIndex->virtualIndex fix this would incorrectly fire with 1
    // (id=1's filtered index), landing on the divider instead of the row.
    expect(scrollToIndex).toHaveBeenCalledWith(2, expect.objectContaining({ align: 'start' }));
  });

  it('does not inject a divider when session bounds are unknown (no estimated timestamps)', () => {
    const { container } = render(
      <CursorTranscriptPane
        sessionId="s"
        items={items}
        filteredItems={items}
        loading={false}
        error={null}
      />,
    );
    expect(container.textContent).not.toMatch(/\w+day, /);
  });
});
