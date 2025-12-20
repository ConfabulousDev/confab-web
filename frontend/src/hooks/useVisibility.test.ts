import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useVisibility } from './useVisibility';

describe('useVisibility', () => {
  let originalVisibilityState: PropertyDescriptor | undefined;

  beforeEach(() => {
    // Store the original visibilityState descriptor
    originalVisibilityState = Object.getOwnPropertyDescriptor(document, 'visibilityState');
  });

  afterEach(() => {
    // Restore the original visibilityState descriptor
    if (originalVisibilityState) {
      Object.defineProperty(document, 'visibilityState', originalVisibilityState);
    } else {
      // @ts-expect-error - restoring original state
      delete document.visibilityState;
    }
    vi.restoreAllMocks();
  });

  it('returns true when document is visible', () => {
    Object.defineProperty(document, 'visibilityState', {
      value: 'visible',
      configurable: true,
    });

    const { result } = renderHook(() => useVisibility());

    expect(result.current).toBe(true);
  });

  it('returns false when document is hidden', () => {
    Object.defineProperty(document, 'visibilityState', {
      value: 'hidden',
      configurable: true,
    });

    const { result } = renderHook(() => useVisibility());

    expect(result.current).toBe(false);
  });

  it('updates when visibility changes from visible to hidden', () => {
    Object.defineProperty(document, 'visibilityState', {
      value: 'visible',
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useVisibility());

    expect(result.current).toBe(true);

    // Simulate visibility change
    act(() => {
      Object.defineProperty(document, 'visibilityState', {
        value: 'hidden',
        writable: true,
        configurable: true,
      });
      document.dispatchEvent(new Event('visibilitychange'));
    });

    expect(result.current).toBe(false);
  });

  it('updates when visibility changes from hidden to visible', () => {
    Object.defineProperty(document, 'visibilityState', {
      value: 'hidden',
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useVisibility());

    expect(result.current).toBe(false);

    // Simulate visibility change
    act(() => {
      Object.defineProperty(document, 'visibilityState', {
        value: 'visible',
        writable: true,
        configurable: true,
      });
      document.dispatchEvent(new Event('visibilitychange'));
    });

    expect(result.current).toBe(true);
  });

  it('cleans up event listener on unmount', () => {
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener');

    Object.defineProperty(document, 'visibilityState', {
      value: 'visible',
      configurable: true,
    });

    const { unmount } = renderHook(() => useVisibility());

    unmount();

    expect(removeEventListenerSpy).toHaveBeenCalledWith(
      'visibilitychange',
      expect.any(Function)
    );
  });
});
