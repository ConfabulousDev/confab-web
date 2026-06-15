import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useApiData } from './useApiData';

interface Params {
  q?: string;
}

/** A typed fetch mock so `P` is inferred as `Params` without `as` casts. */
function makeFetch(impl: () => Promise<{ value: number }>) {
  return vi.fn<(params: Params) => Promise<{ value: number }>>(impl);
}

describe('useApiData', () => {
  it('fetches on mount with initial params; loading → data', async () => {
    const fetchFn = makeFetch(() => Promise.resolve({ value: 1 }));
    const { result } = renderHook(() => useApiData(fetchFn, { q: 'init' }, 'failed'));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual({ value: 1 });
    expect(result.current.error).toBeNull();
    expect(fetchFn).toHaveBeenCalledWith({ q: 'init' });
  });

  it('captures an Error rejection verbatim', async () => {
    const fetchFn = makeFetch(() => Promise.reject(new Error('boom')));
    const { result } = renderHook(() => useApiData(fetchFn, {}, 'failed'));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('boom');
    expect(result.current.data).toBeNull();
  });

  it('wraps a non-Error rejection in Error(errorMessage)', async () => {
    const fetchFn = makeFetch(() => Promise.reject('nope'));
    const { result } = renderHook(() => useApiData(fetchFn, {}, 'fallback message'));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error?.message).toBe('fallback message');
  });

  it('refetch() with no args re-uses the captured initial params', async () => {
    const fetchFn = makeFetch(() => Promise.resolve({ value: 1 }));
    const { result } = renderHook(() => useApiData(fetchFn, { q: 'init' }, 'failed'));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.refetch();
    expect(fetchFn).toHaveBeenLastCalledWith({ q: 'init' });
  });

  it('refetch(newParams) fetches with the new params', async () => {
    const fetchFn = makeFetch(() => Promise.resolve({ value: 1 }));
    const { result } = renderHook(() => useApiData(fetchFn, { q: 'init' }, 'failed'));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.refetch({ q: 'next' });
    expect(fetchFn).toHaveBeenLastCalledWith({ q: 'next' });
  });
});
