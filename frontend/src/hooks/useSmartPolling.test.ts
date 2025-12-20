import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useSmartPolling } from './useSmartPolling';

// Mock the visibility and activity hooks
vi.mock('./useVisibility', () => ({
  useVisibility: vi.fn(() => true),
}));

vi.mock('./useUserActivity', () => ({
  useUserActivity: vi.fn(() => ({ isIdle: false, markActive: vi.fn() })),
}));

import { useVisibility } from './useVisibility';
import { useUserActivity } from './useUserActivity';

describe('useSmartPolling', () => {
  beforeEach(() => {
    vi.mocked(useVisibility).mockReturnValue(true);
    vi.mocked(useUserActivity).mockReturnValue({ isIdle: false, markActive: vi.fn() });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns initial state correctly', () => {
    const fetchFn = vi.fn(() => new Promise<string>(() => {})); // Never resolves

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    expect(result.current.data).toBeNull();
    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.state).toBe('active');
  });

  it('fetches data immediately when visible', async () => {
    const fetchFn = vi.fn(() => Promise.resolve('test-data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(fetchFn).toHaveBeenCalled();
    expect(result.current.data).toBe('test-data');
  });

  it('returns suspended state when not visible', () => {
    vi.mocked(useVisibility).mockReturnValue(false);

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    expect(result.current.state).toBe('suspended');
  });

  it('returns passive state when visible but idle', () => {
    vi.mocked(useVisibility).mockReturnValue(true);
    vi.mocked(useUserActivity).mockReturnValue({ isIdle: true, markActive: vi.fn() });

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    expect(result.current.state).toBe('passive');
  });

  it('returns active state when visible and not idle', () => {
    vi.mocked(useVisibility).mockReturnValue(true);
    vi.mocked(useUserActivity).mockReturnValue({ isIdle: false, markActive: vi.fn() });

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    expect(result.current.state).toBe('active');
  });

  it('does not poll when disabled', async () => {
    const fetchFn = vi.fn(() => Promise.resolve('data'));

    renderHook(() => useSmartPolling(fetchFn, { enabled: false }));

    // Wait a bit to make sure no fetch happens
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(fetchFn).not.toHaveBeenCalled();
  });

  it('does not poll when not visible', async () => {
    vi.mocked(useVisibility).mockReturnValue(false);

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    renderHook(() => useSmartPolling(fetchFn));

    // Wait a bit to make sure no fetch happens
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(fetchFn).not.toHaveBeenCalled();
  });

  it('handles null return (no change) without updating data', async () => {
    let callCount = 0;
    const fetchFn = vi.fn(() => {
      callCount++;
      if (callCount === 1) return Promise.resolve('initial-data');
      return Promise.resolve(null); // No change
    });

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    await waitFor(() => {
      expect(result.current.data).toBe('initial-data');
    });

    // Manually call refetch which returns null
    await act(async () => {
      await result.current.refetch();
    });

    // Data should remain unchanged
    expect(result.current.data).toBe('initial-data');
  });

  it('handles errors gracefully', async () => {
    const fetchFn = vi.fn(() => Promise.reject(new Error('Network error')));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('Network error');
  });

  it('refetch function triggers immediate fetch', async () => {
    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    const callCountAfterInitial = fetchFn.mock.calls.length;

    await act(async () => {
      await result.current.refetch();
    });

    expect(fetchFn.mock.calls.length).toBe(callCountAfterInitial + 1);
  });

  it('uses merge function when provided', async () => {
    let callCount = 0;
    const fetchFn = vi.fn(() => {
      callCount++;
      return Promise.resolve([`item-${callCount}`]);
    });

    const merge = (prev: string[] | null, next: string[]) => {
      return prev ? [...prev, ...next] : next;
    };

    const { result } = renderHook(() =>
      useSmartPolling(fetchFn, { merge })
    );

    // Wait for initial fetch
    await waitFor(() => {
      expect(result.current.data).toEqual(['item-1']);
    });

    // Trigger another fetch via refetch
    await act(async () => {
      await result.current.refetch();
    });

    // Data should be merged
    expect(result.current.data).toEqual(['item-1', 'item-2']);
  });

  it('fetches immediately when becoming visible', async () => {
    vi.mocked(useVisibility).mockReturnValue(false);

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { rerender } = renderHook(() => useSmartPolling(fetchFn));

    // Should not have fetched while hidden
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(fetchFn).not.toHaveBeenCalled();

    // Become visible
    vi.mocked(useVisibility).mockReturnValue(true);
    rerender();

    await waitFor(() => {
      expect(fetchFn).toHaveBeenCalled();
    });
  });

  it('uses active interval when user is not idle', () => {
    vi.mocked(useUserActivity).mockReturnValue({ isIdle: false, markActive: vi.fn() });

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    // The state should be active, meaning active interval is used
    expect(result.current.state).toBe('active');
    // The actual interval is POLLING_CONFIG.ACTIVE_INTERVAL_MS (30s)
  });

  it('uses passive interval when user is idle', () => {
    vi.mocked(useUserActivity).mockReturnValue({ isIdle: true, markActive: vi.fn() });

    const fetchFn = vi.fn(() => Promise.resolve('data'));

    const { result } = renderHook(() => useSmartPolling(fetchFn));

    // The state should be passive, meaning passive interval is used
    expect(result.current.state).toBe('passive');
    // The actual interval is POLLING_CONFIG.PASSIVE_INTERVAL_MS (60s)
  });
});
