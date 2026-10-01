// 6h7m: unit tests for the shared day-boundary/idle-gap divider decision and
// label functions. These are the single seam all 4 providers' *VirtualItems
// builders funnel through, so correctness here is load-bearing for every
// provider's divider placement.
//
// cca0: also covers the virtualized-scroll helpers the 4 transcript panes share
// (search-match scroll, top/bottom scroll, bar-seek index scan).

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import type { VirtualItem } from '@tanstack/react-virtual';
import {
  shouldShowDivider,
  formatDividerLabel,
  TIME_GAP_THRESHOLD_MS,
  scrollToSearchMatch,
  scrollVirtualizerToStart,
  scrollVirtualizerToEnd,
  firstIndexAtOrAfter,
} from './timelineUtils';

// Constructed with the local-time Date(year, month, day, ...) form (NOT ISO
// 'Z' strings) throughout this file: day-boundary math is local-calendar-day
// based (matching the existing formatTimeSeparator convention), so a fixture
// built from a UTC string would shift days depending on the test runner's TZ.
const MAY_13_18_00 = new Date(2026, 4, 13, 18, 0, 0).getTime();

describe('shouldShowDivider', () => {
  it('does not show a divider when there is no previous known timestamp (first item)', () => {
    const result = shouldShowDivider(MAY_13_18_00, undefined);
    expect(result).toEqual({ show: false, dayChanged: false });
  });

  it('does not show a divider for a same-day gap under the 5min threshold', () => {
    const result = shouldShowDivider(MAY_13_18_00, MAY_13_18_00 - 60_000);
    expect(result).toEqual({ show: false, dayChanged: false });
  });

  it('shows a divider (no day change) for a same-day gap over the 5min threshold', () => {
    const result = shouldShowDivider(MAY_13_18_00, MAY_13_18_00 - (TIME_GAP_THRESHOLD_MS + 1));
    expect(result).toEqual({ show: true, dayChanged: false });
  });

  it('does not show a divider for a same-day gap exactly at the 5min threshold', () => {
    const result = shouldShowDivider(MAY_13_18_00, MAY_13_18_00 - TIME_GAP_THRESHOLD_MS);
    expect(result).toEqual({ show: false, dayChanged: false });
  });

  it('shows a divider with dayChanged=true when the calendar day changes even with a tiny gap', () => {
    // 11:59pm -> 12:01am: a 2-minute gap that crosses midnight.
    const may13_2359 = new Date(2026, 4, 13, 23, 59, 0).getTime();
    const may14_0001 = new Date(2026, 4, 14, 0, 1, 0).getTime();
    const result = shouldShowDivider(may14_0001, may13_2359);
    expect(result).toEqual({ show: true, dayChanged: true });
  });

  it('shows a divider when the day changes even with a very large gap', () => {
    const dayLater = MAY_13_18_00 + 24 * 60 * 60 * 1000;
    const result = shouldShowDivider(dayLater, MAY_13_18_00);
    expect(result).toEqual({ show: true, dayChanged: true });
  });

  it('does not show a divider for two timestamps on the same day far apart intraday', () => {
    const morning = new Date(2026, 4, 13, 0, 5, 0).getTime();
    const evening = new Date(2026, 4, 13, 0, 10, 0).getTime();
    const result = shouldShowDivider(evening, morning);
    expect(result).toEqual({ show: false, dayChanged: false });
  });
});

describe('formatDividerLabel', () => {
  it('returns a full weekday+month+day label when dayChanged is true', () => {
    const label = formatDividerLabel(new Date(2026, 6, 7, 0, 1, 0).getTime(), true);
    // e.g. "Tuesday, July 7" - locale-dependent exact format, so check the pieces.
    expect(label).toMatch(/\w+day/); // contains a weekday name
    expect(label).toContain('July');
    expect(label).toContain('7');
  });

  it('does not include a weekday/full-date format when dayChanged is false', () => {
    const label = formatDividerLabel(new Date().getTime(), false);
    // Falls back to the existing idle-gap time-only/short-date text, which
    // never contains a full weekday name.
    expect(label).not.toMatch(/day,/);
  });
});

// Manual animation-frame queue: each `flushFrame()` runs exactly the callbacks
// scheduled before it, so tests can count frames the way the browser would.
let frameQueue: FrameRequestCallback[] = [];
function flushFrame(): void {
  const batch = frameQueue;
  frameQueue = [];
  batch.forEach((cb) => cb(0));
}
function flushFrames(n: number): void {
  for (let i = 0; i < n; i++) flushFrame();
}

function vItem(index: number): VirtualItem {
  return { key: index, index, start: 0, end: 0, size: 0, lane: 0 };
}

// The real `getVirtualItems` is a memoized fn that also carries an
// `updateDeps` member (never called by the helpers), so the fake mirrors it.
function fakeVirtualizer(getVisible: () => VirtualItem[]) {
  const scrollToIndex = vi.fn();
  const virtualizer = {
    scrollToIndex,
    getVirtualItems: Object.assign(getVisible, { updateDeps: () => {} }),
  };
  return { virtualizer, scrollToIndex };
}

describe('scroll helpers (cca0)', () => {
  beforeEach(() => {
    frameQueue = [];
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      frameQueue.push(cb);
      return frameQueue.length;
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.innerHTML = '';
  });

  describe('scrollToSearchMatch', () => {
    function setup(opts: { rowPresent: boolean; withMark: boolean }) {
      const scrollEl = document.createElement('div');
      document.body.appendChild(scrollEl);
      const scrollIntoView = vi.fn();
      const addRow = (withMark: boolean) => {
        const row = document.createElement('div');
        row.setAttribute('data-index', '3');
        if (withMark) {
          const mark = document.createElement('mark');
          mark.scrollIntoView = scrollIntoView;
          row.appendChild(mark);
        }
        scrollEl.appendChild(row);
        return row;
      };
      if (opts.rowPresent) addRow(opts.withMark);
      const virtualizer = { scrollToIndex: vi.fn() };
      return { scrollEl, scrollIntoView, addRow, virtualizer };
    }

    it('scrolls the row to center, then brings its first <mark> into view after the delay', () => {
      const { scrollEl, scrollIntoView, virtualizer } = setup({ rowPresent: true, withMark: true });
      scrollToSearchMatch(virtualizer, () => scrollEl, 3);

      expect(virtualizer.scrollToIndex).toHaveBeenCalledWith(3, { align: 'center' });
      // The 6-frame settle delay: no mark scroll before it elapses.
      flushFrames(5);
      expect(scrollIntoView).not.toHaveBeenCalled();
      flushFrame();
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
      expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest', behavior: 'smooth' });
      flushFrames(30);
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
      // scrollToIndex was retried across frames as measurements settle.
      expect(virtualizer.scrollToIndex.mock.calls.length).toBeGreaterThan(1);
      for (const call of virtualizer.scrollToIndex.mock.calls) {
        expect(call).toEqual([3, { align: 'center' }]);
      }
    });

    it('still scrolls the mark when the row only mounts a few frames after the delay', () => {
      const { scrollEl, scrollIntoView, addRow, virtualizer } = setup({ rowPresent: false, withMark: false });
      scrollToSearchMatch(virtualizer, () => scrollEl, 3);
      flushFrames(9);
      expect(scrollIntoView).not.toHaveBeenCalled();
      addRow(true);
      flushFrame();
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
    });

    it('still scrolls the mark on the 10th and final attempt', () => {
      // Attempt k runs on frame 6 + k, so attempt 9 (the last) is frame 15.
      const { scrollEl, scrollIntoView, addRow, virtualizer } = setup({ rowPresent: false, withMark: false });
      scrollToSearchMatch(virtualizer, () => scrollEl, 3);
      flushFrames(14);
      addRow(true);
      flushFrame();
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
    });

    it('gives up after 10 attempts with no throw when no <mark> ever appears', () => {
      const { scrollEl, scrollIntoView, addRow, virtualizer } = setup({ rowPresent: true, withMark: false });
      scrollToSearchMatch(virtualizer, () => scrollEl, 3);
      expect(() => flushFrames(16)).not.toThrow();
      // Out of attempts: a mark arriving now is never scrolled, and no more
      // frames are being requested.
      scrollEl.innerHTML = '';
      addRow(true);
      flushFrames(30);
      expect(scrollIntoView).not.toHaveBeenCalled();
      expect(frameQueue).toHaveLength(0);
    });

    it('never scrolls the mark when cancelled before the delay elapses', () => {
      const { scrollEl, scrollIntoView, virtualizer } = setup({ rowPresent: true, withMark: true });
      const cancel = scrollToSearchMatch(virtualizer, () => scrollEl, 3);
      flushFrames(3);
      cancel();
      flushFrames(30);
      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('does not throw or scroll when the scroll element is gone', () => {
      const { scrollIntoView, virtualizer } = setup({ rowPresent: true, withMark: true });
      scrollToSearchMatch(virtualizer, () => null, 3);
      expect(() => flushFrames(30)).not.toThrow();
      expect(scrollIntoView).not.toHaveBeenCalled();
    });
  });

  describe('scrollVirtualizerToStart', () => {
    it('retries scrolling to index 0 until the first visible item is index 0, then stops', () => {
      let visible = [vItem(4), vItem(5)];
      const { virtualizer, scrollToIndex } = fakeVirtualizer(() => visible);
      scrollVirtualizerToStart(virtualizer);
      expect(scrollToIndex).toHaveBeenCalledTimes(1);
      expect(scrollToIndex).toHaveBeenCalledWith(0, { align: 'start' });
      flushFrame();
      expect(scrollToIndex).toHaveBeenCalledTimes(2);
      visible = [vItem(0), vItem(1)];
      flushFrame();
      expect(scrollToIndex).toHaveBeenCalledTimes(3);
      flushFrames(10);
      expect(scrollToIndex).toHaveBeenCalledTimes(3);
    });

    it('stops after a bounded number of retries when index 0 never becomes visible', () => {
      const { virtualizer, scrollToIndex } = fakeVirtualizer(() => []);
      scrollVirtualizerToStart(virtualizer);
      flushFrames(50);
      expect(scrollToIndex).toHaveBeenCalledTimes(6);
      expect(frameQueue).toHaveLength(0);
    });
  });

  describe('scrollVirtualizerToEnd', () => {
    it('retries scrolling to the last index until it is visible, then stops', () => {
      let visible = [vItem(0), vItem(1)];
      const { virtualizer, scrollToIndex } = fakeVirtualizer(() => visible);
      scrollVirtualizerToEnd(virtualizer, 9);
      expect(scrollToIndex).toHaveBeenCalledWith(9, { align: 'end' });
      flushFrame();
      expect(scrollToIndex).toHaveBeenCalledTimes(2);
      visible = [vItem(8), vItem(9)];
      flushFrame();
      flushFrames(10);
      expect(scrollToIndex).toHaveBeenCalledTimes(3);
    });

    it('does not throw for an empty list (lastIndex -1)', () => {
      const { virtualizer } = fakeVirtualizer(() => []);
      expect(() => {
        scrollVirtualizerToEnd(virtualizer, -1);
        flushFrames(50);
      }).not.toThrow();
      expect(frameQueue).toHaveLength(0);
    });
  });
});

describe('firstIndexAtOrAfter (cca0)', () => {
  const map = new Map<string, number>([
    ['b', 0],
    ['d', 1],
  ]);
  const items = ['a', 'b', 'c', 'd'];
  const lookup = (it: string) => map.get(it);

  it('finds the first mapped item at the start index', () => {
    expect(firstIndexAtOrAfter(items, 1, lookup)).toBe(0);
  });

  it('skips unmapped items', () => {
    expect(firstIndexAtOrAfter(items, 0, lookup)).toBe(0);
    expect(firstIndexAtOrAfter(items, 2, lookup)).toBe(1);
  });

  it('returns undefined when nothing at or after start maps', () => {
    expect(firstIndexAtOrAfter(['a', 'c'], 0, lookup)).toBeUndefined();
  });

  it('returns undefined when start is at or past the end', () => {
    expect(firstIndexAtOrAfter(items, 4, lookup)).toBeUndefined();
    expect(firstIndexAtOrAfter(items, 99, lookup)).toBeUndefined();
  });

  it('handles start 0 on an empty array', () => {
    expect(firstIndexAtOrAfter([], 0, lookup)).toBeUndefined();
  });

  it('skips holes in a sparse array', () => {
    const sparse: string[] = [];
    sparse[3] = 'd';
    expect(firstIndexAtOrAfter(sparse, 0, lookup)).toBe(1);
  });
});
