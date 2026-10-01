// cca0: the shared first-visible-row tracker every transcript pane feeds into
// its TimelineBar position indicator.

import { describe, it, expect, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import type { VirtualItem } from '@tanstack/react-virtual';
import { useFirstVisibleIndex } from './useFirstVisibleIndex';

function vItem(index: number): VirtualItem {
  return { key: index, index, start: 0, end: 0, size: 0, lane: 0 };
}

const rows = [
  { type: 'separator' },
  { type: 'item', index: 5 },
  { type: 'separator' },
  { type: 'item', index: 6 },
];

function setup(initialVisible: number[]) {
  let visible = initialVisible.map(vItem);
  // The real `getVirtualItems` is a memoized fn that also carries an
  // `updateDeps` member (never called by the hook), so the fake mirrors it.
  const getVirtualItems = Object.assign(vi.fn(() => visible), { updateDeps: () => {} });
  const virtualizer = { getVirtualItems };
  const el = document.createElement('div');
  const scrollRef = { current: el };
  const setVisible = (next: number[]) => {
    visible = next.map(vItem);
  };
  return { virtualizer, getVirtualItems, el, scrollRef, setVisible };
}

describe('useFirstVisibleIndex', () => {
  it('reports the first visible row of rowType on attach, skipping leading separator rows', () => {
    const { virtualizer, scrollRef } = setup([0, 1, 2, 3]);
    const { result } = renderHook(() =>
      useFirstVisibleIndex(scrollRef, virtualizer, rows, 'item'),
    );
    expect(result.current).toBe(5);
  });

  it('updates on a scroll event', () => {
    const { virtualizer, el, scrollRef, setVisible } = setup([0, 1]);
    const { result } = renderHook(() =>
      useFirstVisibleIndex(scrollRef, virtualizer, rows, 'item'),
    );
    expect(result.current).toBe(5);
    setVisible([2, 3]);
    act(() => {
      el.dispatchEvent(new Event('scroll'));
    });
    expect(result.current).toBe(6);
  });

  it('only counts rows whose type matches rowType', () => {
    const { virtualizer, scrollRef } = setup([0, 1, 2, 3]);
    const { result } = renderHook(() =>
      useFirstVisibleIndex(
        scrollRef,
        virtualizer,
        [{ type: 'item', index: 2 }, { type: 'message', index: 9 }],
        'message',
      ),
    );
    expect(result.current).toBe(9);
  });

  it('removes the scroll listener on unmount', () => {
    const { virtualizer, getVirtualItems, el, scrollRef } = setup([0, 1]);
    const removeSpy = vi.spyOn(el, 'removeEventListener');
    const { unmount } = renderHook(() =>
      useFirstVisibleIndex(scrollRef, virtualizer, rows, 'item'),
    );
    unmount();
    expect(removeSpy).toHaveBeenCalledWith('scroll', expect.any(Function));
    getVirtualItems.mockClear();
    el.dispatchEvent(new Event('scroll'));
    expect(getVirtualItems).not.toHaveBeenCalled();
  });

  it('stays 0 when no row of rowType is visible', () => {
    const { virtualizer, el, scrollRef, setVisible } = setup([0]);
    const { result } = renderHook(() =>
      useFirstVisibleIndex(scrollRef, virtualizer, rows, 'item'),
    );
    expect(result.current).toBe(0);
    setVisible([]);
    act(() => {
      el.dispatchEvent(new Event('scroll'));
    });
    expect(result.current).toBe(0);
  });

  it('keeps the last reported index when a scroll leaves only separators visible', () => {
    const { virtualizer, el, scrollRef, setVisible } = setup([1]);
    const { result } = renderHook(() =>
      useFirstVisibleIndex(scrollRef, virtualizer, rows, 'item'),
    );
    expect(result.current).toBe(5);
    setVisible([2]);
    act(() => {
      el.dispatchEvent(new Event('scroll'));
    });
    expect(result.current).toBe(5);
  });

  it('handles a null scroll ref without throwing', () => {
    const { virtualizer } = setup([0, 1]);
    const { result } = renderHook(() =>
      useFirstVisibleIndex({ current: null }, virtualizer, rows, 'item'),
    );
    expect(result.current).toBe(0);
  });
});
