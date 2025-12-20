import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useUserActivity } from './useUserActivity';
import { POLLING_CONFIG } from '@/config/polling';

describe('useUserActivity', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('starts with isIdle as false', () => {
    const { result } = renderHook(() => useUserActivity());

    expect(result.current.isIdle).toBe(false);
  });

  it('becomes idle after threshold period without activity', () => {
    const { result } = renderHook(() => useUserActivity());

    expect(result.current.isIdle).toBe(false);

    // Advance time past the idle threshold plus the check interval
    act(() => {
      vi.advanceTimersByTime(POLLING_CONFIG.IDLE_THRESHOLD_MS + 5000);
    });

    expect(result.current.isIdle).toBe(true);
  });

  it('stays active when user activity events occur', () => {
    const { result } = renderHook(() => useUserActivity());

    // Advance time close to the threshold
    act(() => {
      vi.advanceTimersByTime(POLLING_CONFIG.IDLE_THRESHOLD_MS - 1000);
    });

    // Simulate user activity
    act(() => {
      document.dispatchEvent(new MouseEvent('click'));
    });

    // Advance time past where we would have become idle
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    // Should still be active because we clicked
    expect(result.current.isIdle).toBe(false);
  });

  it('responds to different activity event types', () => {
    const { result } = renderHook(() => useUserActivity());

    // Advance time past threshold
    act(() => {
      vi.advanceTimersByTime(POLLING_CONFIG.IDLE_THRESHOLD_MS + 5000);
    });

    expect(result.current.isIdle).toBe(true);

    // Test keydown event
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown'));
    });

    expect(result.current.isIdle).toBe(false);

    // Advance time past threshold again
    act(() => {
      vi.advanceTimersByTime(POLLING_CONFIG.IDLE_THRESHOLD_MS + 5000);
    });

    expect(result.current.isIdle).toBe(true);

    // Test scroll event
    act(() => {
      document.dispatchEvent(new Event('scroll'));
    });

    expect(result.current.isIdle).toBe(false);
  });

  it('markActive function resets idle state', () => {
    const { result } = renderHook(() => useUserActivity());

    // Become idle
    act(() => {
      vi.advanceTimersByTime(POLLING_CONFIG.IDLE_THRESHOLD_MS + 5000);
    });

    expect(result.current.isIdle).toBe(true);

    // Manually mark active
    act(() => {
      result.current.markActive();
    });

    expect(result.current.isIdle).toBe(false);
  });

  it('cleans up event listeners and interval on unmount', () => {
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener');

    const { unmount } = renderHook(() => useUserActivity());

    unmount();

    // Should have removed listeners for all activity events
    for (const event of POLLING_CONFIG.ACTIVITY_EVENTS) {
      expect(removeEventListenerSpy).toHaveBeenCalledWith(
        event,
        expect.any(Function)
      );
    }
  });

  it('checks idle status every 5 seconds', () => {
    const { result } = renderHook(() => useUserActivity());

    // After 55 seconds (just under threshold), should still be active
    act(() => {
      vi.advanceTimersByTime(55000);
    });

    expect(result.current.isIdle).toBe(false);

    // After another 5 seconds (now at 60s), the next check should mark idle
    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current.isIdle).toBe(true);
  });
});
