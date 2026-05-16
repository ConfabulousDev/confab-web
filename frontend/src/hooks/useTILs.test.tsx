import { describe, it, expect, beforeEach, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { useTILsFetch } from './useTILs';
import { tilsAPI } from '@/services/api';
import type { TILListResponse } from '@/schemas/api';
import type { SessionFilters } from './useSessionFilters';

vi.mock('@/services/api', () => ({
  tilsAPI: { list: vi.fn(), delete: vi.fn() },
}));

const emptyFilters: SessionFilters = {
  query: '',
  repos: [],
  branches: [],
  owners: [],
  providers: [],
};

function makeResponse(overrides: Partial<TILListResponse> = {}): TILListResponse {
  return {
    tils: [],
    has_more: false,
    next_cursor: '',
    page_size: 20,
    filter_options: { repos: [], branches: [], owners: [] },
    ...overrides,
  };
}

describe('useTILsFetch', () => {
  beforeEach(() => {
    vi.mocked(tilsAPI.list).mockReset();
    vi.mocked(tilsAPI.delete).mockReset();
  });

  it('fetches on mount with no params when filters are empty', async () => {
    vi.mocked(tilsAPI.list).mockResolvedValue(makeResponse());

    const { result } = renderHook(() => useTILsFetch(emptyFilters));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(tilsAPI.list).toHaveBeenCalledTimes(1);
    expect(tilsAPI.list).toHaveBeenCalledWith(undefined);
  });

  it('flattens array filters to comma-joined query params', async () => {
    vi.mocked(tilsAPI.list).mockResolvedValue(makeResponse());

    const filters: SessionFilters = {
      query: 'hello',
      repos: ['repo-a', 'repo-b'],
      branches: ['main'],
      owners: ['alice@example.com'],
      providers: [],
    };

    const { result } = renderHook(() => useTILsFetch(filters));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(tilsAPI.list).toHaveBeenCalledWith({
      repo: 'repo-a,repo-b',
      branch: 'main',
      owner: 'alice@example.com',
      q: 'hello',
    });
  });

  it('debounces search query changes by 300ms', async () => {
    vi.useFakeTimers();
    try {
      vi.mocked(tilsAPI.list).mockResolvedValue(makeResponse());

      const { rerender } = renderHook(({ f }: { f: SessionFilters }) => useTILsFetch(f), {
        initialProps: { f: emptyFilters },
      });

      // Initial mount fetch is non-debounced; flush microtasks.
      await act(() => vi.advanceTimersByTimeAsync(0));
      expect(tilsAPI.list).toHaveBeenCalledTimes(1);

      rerender({ f: { ...emptyFilters, query: 'a' } });
      rerender({ f: { ...emptyFilters, query: 'ab' } });
      rerender({ f: { ...emptyFilters, query: 'abc' } });

      await act(() => vi.advanceTimersByTimeAsync(299));
      expect(tilsAPI.list).toHaveBeenCalledTimes(1);

      await act(() => vi.advanceTimersByTimeAsync(1));
      expect(tilsAPI.list).toHaveBeenCalledTimes(2);
      expect(tilsAPI.list).toHaveBeenLastCalledWith({ q: 'abc' });
    } finally {
      vi.useRealTimers();
    }
  });

  it('goNext advances cursor and enables canGoPrev', async () => {
    vi.mocked(tilsAPI.list)
      .mockResolvedValueOnce(makeResponse({ has_more: true, next_cursor: 'cursor-1' }))
      .mockResolvedValueOnce(makeResponse({ has_more: true, next_cursor: 'cursor-2' }));

    const { result } = renderHook(() => useTILsFetch(emptyFilters));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.canGoPrev).toBe(false);

    act(() => {
      result.current.goNext();
    });
    await waitFor(() => expect(tilsAPI.list).toHaveBeenCalledTimes(2));
    expect(tilsAPI.list).toHaveBeenLastCalledWith({ cursor: 'cursor-1' });
    expect(result.current.canGoPrev).toBe(true);
  });

  it('goPrev restores prior cursor and pops the stack', async () => {
    vi.mocked(tilsAPI.list)
      .mockResolvedValueOnce(makeResponse({ has_more: true, next_cursor: 'cursor-1' }))
      .mockResolvedValueOnce(makeResponse({ has_more: true, next_cursor: 'cursor-2' }))
      .mockResolvedValueOnce(makeResponse({ has_more: true, next_cursor: 'cursor-1' }));

    const { result } = renderHook(() => useTILsFetch(emptyFilters));
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.goNext());
    await waitFor(() => expect(tilsAPI.list).toHaveBeenCalledTimes(2));
    expect(result.current.canGoPrev).toBe(true);

    act(() => result.current.goPrev());
    await waitFor(() => expect(tilsAPI.list).toHaveBeenCalledTimes(3));
    expect(tilsAPI.list).toHaveBeenLastCalledWith(undefined);
    expect(result.current.canGoPrev).toBe(false);
  });

  it('goNext is a no-op when hasMore is false', async () => {
    vi.mocked(tilsAPI.list).mockResolvedValue(
      makeResponse({ has_more: false, next_cursor: '' })
    );

    const { result } = renderHook(() => useTILsFetch(emptyFilters));
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.goNext());
    // Give any unintended follow-up fetch a chance to fire.
    await new Promise((r) => setTimeout(r, 10));
    expect(tilsAPI.list).toHaveBeenCalledTimes(1);
    expect(result.current.canGoPrev).toBe(false);
  });

  it('deleteTIL invokes API.delete then refetches', async () => {
    vi.mocked(tilsAPI.list).mockResolvedValue(makeResponse());
    vi.mocked(tilsAPI.delete).mockResolvedValue();

    const { result } = renderHook(() => useTILsFetch(emptyFilters));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.deleteTIL(42);
    });

    expect(tilsAPI.delete).toHaveBeenCalledWith(42);
    expect(tilsAPI.list).toHaveBeenCalledTimes(2);
  });

  it('surfaces API errors as Error instances', async () => {
    vi.mocked(tilsAPI.list).mockRejectedValue(new Error('list-failed'));

    const { result } = renderHook(() => useTILsFetch(emptyFilters));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('list-failed');
  });

  it('wraps non-Error rejections in an Error', async () => {
    vi.mocked(tilsAPI.list).mockRejectedValue('string-failure');

    const { result } = renderHook(() => useTILsFetch(emptyFilters));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error?.message).toBe('Failed to fetch TILs');
  });
});
