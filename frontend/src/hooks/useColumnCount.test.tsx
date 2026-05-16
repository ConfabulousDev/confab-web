import { describe, it, expect, afterEach, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { distributeToColumns, useColumnCount } from './useColumnCount';

function setInnerWidth(value: number) {
  Object.defineProperty(window, 'innerWidth', { value, configurable: true, writable: true });
}

afterEach(() => {
  setInnerWidth(1024);
});

describe('distributeToColumns', () => {
  it('round-robins items left-to-right across N columns', () => {
    const result = distributeToColumns([0, 1, 2, 3, 4], 3);
    expect(result).toEqual([[0, 3], [1, 4], [2]]);
  });

  it('returns N empty arrays when items is empty', () => {
    expect(distributeToColumns([], 4)).toEqual([[], [], [], []]);
  });

  it('handles single column', () => {
    expect(distributeToColumns([1, 2, 3], 1)).toEqual([[1, 2, 3]]);
  });
});

describe('useColumnCount', () => {
  it.each([
    [500, 1],
    [800, 2],
    [1200, 3],
    [1500, 4],
  ])('returns %i columns at width %i', (width, expected) => {
    setInnerWidth(width);
    const { result } = renderHook(() => useColumnCount());
    expect(result.current).toBe(expected);
  });

  it('updates on window resize event', () => {
    setInnerWidth(500);
    const { result } = renderHook(() => useColumnCount());
    expect(result.current).toBe(1);

    act(() => {
      setInnerWidth(1500);
      window.dispatchEvent(new Event('resize'));
    });
    expect(result.current).toBe(4);
  });

  it('removes resize listener on unmount', () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener');
    const { unmount } = renderHook(() => useColumnCount());
    unmount();
    expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function));
  });
});
