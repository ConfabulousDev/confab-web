import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAutoRetry } from './useAutoRetry';

describe('useAutoRetry', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does nothing when disabled', () => {
    const retryFn = vi.fn();
    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 10_000,
        maxDelay: 60_000,
        enabled: false,
      }),
    );

    expect(result.current.countdown).toBe(0);
    expect(result.current.attempt).toBe(0);
    expect(result.current.isRetrying).toBe(false);
    expect(result.current.exhausted).toBe(false);
    expect(retryFn).not.toHaveBeenCalled();
  });

  it('starts countdown when enabled', () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 10_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    // Initial countdown should be 10 (10_000ms / 1000)
    expect(result.current.countdown).toBe(10);
    expect(result.current.attempt).toBe(0);
  });

  it('decrements countdown each second', () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 5_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    expect(result.current.countdown).toBe(5);

    act(() => { vi.advanceTimersByTime(1000); });
    expect(result.current.countdown).toBe(4);

    act(() => { vi.advanceTimersByTime(1000); });
    expect(result.current.countdown).toBe(3);
  });

  it('calls retryFn when countdown reaches zero', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 3_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    // Advance past the 3-second countdown
    await act(async () => { vi.advanceTimersByTime(3000); });

    expect(retryFn).toHaveBeenCalledTimes(1);
  });

  it('applies exponential backoff on subsequent retries', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 1_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    // First countdown: 1s (1000 * 2^0 = 1000ms)
    expect(result.current.countdown).toBe(1);
    await act(async () => { vi.advanceTimersByTime(1000); });
    expect(retryFn).toHaveBeenCalledTimes(1);
    expect(result.current.attempt).toBe(1);

    // Second countdown: 2s (1000 * 2^1 = 2000ms)
    expect(result.current.countdown).toBe(2);
    await act(async () => { vi.advanceTimersByTime(2000); });
    expect(retryFn).toHaveBeenCalledTimes(2);
    expect(result.current.attempt).toBe(2);

    // Third countdown: 4s (1000 * 2^2 = 4000ms)
    expect(result.current.countdown).toBe(4);
  });

  it('caps delay at maxDelay', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 10_000,
        maxDelay: 15_000,
        enabled: true,
      }),
    );

    // First countdown: 10s (10000 * 2^0 = 10000ms)
    expect(result.current.countdown).toBe(10);
    await act(async () => { vi.advanceTimersByTime(10_000); });

    // Second countdown: min(20000, 15000) = 15s
    expect(result.current.countdown).toBe(15);
  });

  it('sets exhausted after maxAttempts', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 2,
        initialDelay: 1_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    // Attempt 1
    await act(async () => { vi.advanceTimersByTime(1000); });
    expect(result.current.attempt).toBe(1);
    expect(result.current.exhausted).toBe(false);

    // Attempt 2 (max)
    await act(async () => { vi.advanceTimersByTime(2000); });
    expect(result.current.attempt).toBe(2);
    expect(result.current.exhausted).toBe(true);
    expect(result.current.countdown).toBe(0);
  });

  it('stops retrying after exhaustion', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 1,
        initialDelay: 1_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    await act(async () => { vi.advanceTimersByTime(1000); });
    expect(retryFn).toHaveBeenCalledTimes(1);

    // Advance further — no more calls
    await act(async () => { vi.advanceTimersByTime(60_000); });
    expect(retryFn).toHaveBeenCalledTimes(1);
  });

  it('sets isRetrying while retryFn is executing', async () => {
    let resolveRetry: () => void;
    const retryFn = vi.fn().mockImplementation(
      () => new Promise<void>((resolve) => { resolveRetry = resolve; }),
    );

    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 1_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    // Trigger the retry
    await act(async () => { vi.advanceTimersByTime(1000); });
    expect(result.current.isRetrying).toBe(true);

    // Resolve the retry (success case - component would unmount)
    await act(async () => { resolveRetry!(); });
    // On success, isRetrying stays true since the hook consumer unmounts
  });

  it('resets state when disabled', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) =>
        useAutoRetry(retryFn, {
          maxAttempts: 10,
          initialDelay: 5_000,
          maxDelay: 60_000,
          enabled,
        }),
      { initialProps: { enabled: true } },
    );

    expect(result.current.countdown).toBe(5);

    // Disable
    rerender({ enabled: false });
    expect(result.current.countdown).toBe(0);
    expect(result.current.attempt).toBe(0);
    expect(result.current.exhausted).toBe(false);
  });

  it('cleans up timer on unmount', async () => {
    const retryFn = vi.fn().mockRejectedValue(new Error('fail'));
    const { unmount } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 5_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    unmount();

    // Advancing timers should not cause errors or call retryFn
    await act(async () => { vi.advanceTimersByTime(60_000); });
    expect(retryFn).not.toHaveBeenCalled();
  });

  it('waits for retryFn to complete before starting next countdown', async () => {
    let rejectRetry: (err: Error) => void;
    const retryFn = vi.fn().mockImplementation(
      () => new Promise<void>((_, reject) => { rejectRetry = reject; }),
    );

    const { result } = renderHook(() =>
      useAutoRetry(retryFn, {
        maxAttempts: 10,
        initialDelay: 1_000,
        maxDelay: 60_000,
        enabled: true,
      }),
    );

    // Trigger first retry
    await act(async () => { vi.advanceTimersByTime(1000); });
    expect(result.current.isRetrying).toBe(true);
    expect(result.current.countdown).toBe(0);

    // While retryFn is pending, no new countdown should start
    await act(async () => { vi.advanceTimersByTime(5000); });
    expect(result.current.countdown).toBe(0);
    expect(retryFn).toHaveBeenCalledTimes(1);

    // Reject the retry — next countdown should start
    await act(async () => { rejectRetry!(new Error('fail')); });
    expect(result.current.isRetrying).toBe(false);
    expect(result.current.countdown).toBe(2); // 1000 * 2^1 = 2s
  });
});
