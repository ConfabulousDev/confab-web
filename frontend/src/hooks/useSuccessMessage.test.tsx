import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useSuccessMessage } from './useSuccessMessage';

function routerWrapper(initialEntry: string) {
  return ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[initialEntry]}>{children}</MemoryRouter>
  );
}

describe('useSuccessMessage', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts with empty message and fading=false', () => {
    const { result } = renderHook(() => useSuccessMessage(), { wrapper: routerWrapper('/') });
    expect(result.current.message).toBe('');
    expect(result.current.fading).toBe(false);
  });

  it('setMessage sets message and clears fading', () => {
    const { result } = renderHook(() => useSuccessMessage(), { wrapper: routerWrapper('/') });
    act(() => result.current.setMessage('Saved!'));
    expect(result.current.message).toBe('Saved!');
    expect(result.current.fading).toBe(false);
  });

  it('marks fading=true after fadeDuration', async () => {
    const { result } = renderHook(
      () => useSuccessMessage({ fadeDuration: 1000, clearDuration: 2000 }),
      { wrapper: routerWrapper('/') }
    );
    act(() => result.current.setMessage('hi'));
    expect(result.current.fading).toBe(false);

    await act(() => vi.advanceTimersByTimeAsync(1000));
    expect(result.current.fading).toBe(true);
    expect(result.current.message).toBe('hi');
  });

  it('clears message after clearDuration', async () => {
    const { result } = renderHook(
      () => useSuccessMessage({ fadeDuration: 1000, clearDuration: 2000 }),
      { wrapper: routerWrapper('/') }
    );
    act(() => result.current.setMessage('hi'));

    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(result.current.message).toBe('');
    expect(result.current.fading).toBe(false);
  });

  it('clearMessage resets immediately', () => {
    const { result } = renderHook(() => useSuccessMessage(), { wrapper: routerWrapper('/') });
    act(() => result.current.setMessage('hi'));
    act(() => result.current.clearMessage());
    expect(result.current.message).toBe('');
    expect(result.current.fading).toBe(false);
  });

  it('reads message from URL param on mount and removes it from the URL', () => {
    vi.useRealTimers();
    function ProbeHook() {
      const hook = useSuccessMessage();
      const location = useLocation();
      return { hook, search: location.search };
    }
    const { result } = renderHook(ProbeHook, {
      wrapper: routerWrapper('/dash?success=Saved'),
    });

    expect(result.current.hook.message).toBe('Saved');
    expect(new URLSearchParams(result.current.search).get('success')).toBeNull();
  });

  it('skipUrlParams=true does not consume URL param', () => {
    const { result } = renderHook(
      () => useSuccessMessage({ skipUrlParams: true }),
      { wrapper: routerWrapper('/dash?success=Saved') }
    );
    expect(result.current.message).toBe('');
  });
});
