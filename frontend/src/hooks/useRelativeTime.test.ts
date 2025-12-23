import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useRelativeTime } from './useRelativeTime';

// Mock useVisibility
vi.mock('./useVisibility', () => ({
  useVisibility: vi.fn(() => true),
}));

import { useVisibility } from './useVisibility';

describe('useRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2025-01-15T12:00:00Z'));
    vi.mocked(useVisibility).mockReturnValue(true);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('returns formatted relative time for a recent date', () => {
    // 30 seconds ago
    const date = '2025-01-15T11:59:30Z';
    const { result } = renderHook(() => useRelativeTime(date));

    expect(result.current).toBe('30s ago');
  });

  it('returns formatted relative time for minutes ago', () => {
    // 5 minutes ago
    const date = '2025-01-15T11:55:00Z';
    const { result } = renderHook(() => useRelativeTime(date));

    expect(result.current).toBe('5m ago');
  });

  it('returns formatted relative time for hours ago', () => {
    // 2 hours ago
    const date = '2025-01-15T10:00:00Z';
    const { result } = renderHook(() => useRelativeTime(date));

    expect(result.current).toBe('2h ago');
  });

  it('updates when interval fires for recent timestamps', () => {
    // 30 seconds ago
    const date = '2025-01-15T11:59:30Z';
    const { result } = renderHook(() => useRelativeTime(date));

    expect(result.current).toBe('30s ago');

    // Advance time by 5 seconds
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current).toBe('35s ago');
  });

  it('uses 2 second interval for timestamps less than 5 minutes old', () => {
    const setIntervalSpy = vi.spyOn(global, 'setInterval');

    // 30 seconds ago
    const date = '2025-01-15T11:59:30Z';
    renderHook(() => useRelativeTime(date));

    expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 2000);
  });

  it('uses 5 second interval for timestamps between 5 minutes and 1 hour old', () => {
    const setIntervalSpy = vi.spyOn(global, 'setInterval');

    // 10 minutes ago
    const date = '2025-01-15T11:50:00Z';
    renderHook(() => useRelativeTime(date));

    expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 5000);
  });

  it('uses 60 second interval for timestamps over 1 hour old', () => {
    const setIntervalSpy = vi.spyOn(global, 'setInterval');

    // 2 hours ago
    const date = '2025-01-15T10:00:00Z';
    renderHook(() => useRelativeTime(date));

    expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 60000);
  });

  it('pauses updates when tab is hidden', () => {
    vi.mocked(useVisibility).mockReturnValue(false);

    const setIntervalSpy = vi.spyOn(global, 'setInterval');

    // 30 seconds ago
    const date = '2025-01-15T11:59:30Z';
    renderHook(() => useRelativeTime(date));

    // Should not set up an interval when hidden
    expect(setIntervalSpy).not.toHaveBeenCalled();
  });

  it('cleans up interval on unmount', () => {
    const clearIntervalSpy = vi.spyOn(global, 'clearInterval');

    const date = '2025-01-15T11:59:30Z';
    const { unmount } = renderHook(() => useRelativeTime(date));

    unmount();

    expect(clearIntervalSpy).toHaveBeenCalled();
  });

  it('adjusts interval when timestamp crosses 1 hour threshold', () => {
    const clearIntervalSpy = vi.spyOn(global, 'clearInterval');
    const setIntervalSpy = vi.spyOn(global, 'setInterval');

    // 59 minutes ago - should use 5s interval
    const date = '2025-01-15T11:01:00Z';
    const { result } = renderHook(() => useRelativeTime(date));

    expect(result.current).toBe('59m ago');
    expect(setIntervalSpy).toHaveBeenLastCalledWith(expect.any(Function), 5000);

    // Clear the spy to count new calls
    setIntervalSpy.mockClear();

    // Advance time by 2 minutes - now 61 minutes ago (1h 1m)
    act(() => {
      vi.advanceTimersByTime(120000);
    });

    // Should now show hours and have switched to 60s interval
    expect(result.current).toBe('1h ago');
    // Should have cleared old interval and set new one
    expect(clearIntervalSpy).toHaveBeenCalled();
    expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 60000);
  });

  it('resumes updates when tab becomes visible again', () => {
    const setIntervalSpy = vi.spyOn(global, 'setInterval');

    // Start hidden
    vi.mocked(useVisibility).mockReturnValue(false);

    const date = '2025-01-15T11:59:30Z';
    const { rerender } = renderHook(() => useRelativeTime(date));

    expect(setIntervalSpy).not.toHaveBeenCalled();

    // Become visible
    vi.mocked(useVisibility).mockReturnValue(true);
    rerender();

    expect(setIntervalSpy).toHaveBeenCalled();
  });

  it('resumes updates after going hidden then visible again', () => {
    // Start visible
    vi.mocked(useVisibility).mockReturnValue(true);

    const date = '2025-01-15T11:59:30Z';
    const { result, rerender } = renderHook(() => useRelativeTime(date));

    expect(result.current).toBe('30s ago');

    // Advance time while visible - interval fires and triggers rerender
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(result.current).toBe('35s ago');

    // Go hidden - interval is cleared
    vi.mocked(useVisibility).mockReturnValue(false);
    rerender();

    // Advance time while hidden - no interval, no automatic rerender
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    // Still shows cached value from last render (no rerender occurred)
    expect(result.current).toBe('35s ago');

    // Go visible again - interval should resume
    vi.mocked(useVisibility).mockReturnValue(true);
    rerender();
    // After rerender, formatRelativeTime recalculates: now 40s ago
    expect(result.current).toBe('40s ago');

    // Advance time - should update via interval again
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(result.current).toBe('45s ago');
  });
});
