import { useState, useCallback, useEffect, useRef } from 'react';
import { tilsAPI } from '@/services/api';
import type { TILWithSession, TILFilterOptions } from '@/schemas/api';
import type { SessionFilters } from './useSessionFilters';

interface UseTILsFetchReturn {
  tils: TILWithSession[];
  hasMore: boolean;
  filterOptions: TILFilterOptions | null;
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
  goNext: () => void;
  goPrev: () => void;
  canGoPrev: boolean;
  deleteTIL: (id: number) => Promise<void>;
}

function buildParams(filters: SessionFilters, cursor: string): Record<string, string> {
  const params: Record<string, string> = {};
  if (filters.repos.length > 0) params.repo = filters.repos.join(',');
  if (filters.branches.length > 0) params.branch = filters.branches.join(',');
  if (filters.owners.length > 0) params.owner = filters.owners.join(',');
  if (filters.query) params.q = filters.query;
  if (cursor) params.cursor = cursor;
  return params;
}

/**
 * Hook for fetching the paginated TILs list with server-side filtering.
 * Mirrors useSessionsFetch pattern: cursor-based pagination with debounced search.
 */
export function useTILsFetch(filters: SessionFilters): UseTILsFetchReturn {
  const [tils, setTILs] = useState<TILWithSession[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState('');
  const [filterOptions, setFilterOptions] = useState<TILFilterOptions | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [currentCursor, setCurrentCursor] = useState('');

  const fetchTILs = useCallback(async (params: Record<string, string>) => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);
    try {
      const result = await tilsAPI.list(Object.keys(params).length > 0 ? params : undefined);
      if (controller.signal.aborted) return;
      setTILs(result.tils);
      setHasMore(result.has_more);
      setNextCursor(result.next_cursor || '');
      setFilterOptions(result.filter_options);
    } catch (err) {
      if (controller.signal.aborted) return;
      setError(err instanceof Error ? err : new Error('Failed to fetch TILs'));
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
      }
    }
  }, []);

  const nonQueryKey = JSON.stringify({
    repos: filters.repos,
    branches: filters.branches,
    owners: filters.owners,
  });

  const prevNonQueryKeyRef = useRef(nonQueryKey);
  const prevQueryRef = useRef(filters.query);

  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    if (prevNonQueryKeyRef.current !== nonQueryKey) {
      prevNonQueryKeyRef.current = nonQueryKey;
      setCursorStack([]);
      setCurrentCursor('');
      fetchTILs(buildParams(filters, ''));
    } else {
      fetchTILs(buildParams(filters, currentCursor));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nonQueryKey]);

  useEffect(() => {
    if (prevQueryRef.current === filters.query) return;
    prevQueryRef.current = filters.query;

    setCursorStack([]);
    setCurrentCursor('');

    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }
    debounceRef.current = setTimeout(() => {
      fetchTILs(buildParams(filters, ''));
    }, 300);

    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.query]);

  const cursorChangeRef = useRef(false);
  useEffect(() => {
    if (!cursorChangeRef.current) return;
    cursorChangeRef.current = false;
    fetchTILs(buildParams(filters, currentCursor));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentCursor]);

  const goNext = useCallback(() => {
    if (!hasMore || !nextCursor) return;
    setCursorStack((prev) => [...prev, currentCursor]);
    setCurrentCursor(nextCursor);
    cursorChangeRef.current = true;
  }, [hasMore, nextCursor, currentCursor]);

  const goPrev = useCallback(() => {
    setCursorStack((prev) => {
      if (prev.length === 0) return prev;
      const popped = prev[prev.length - 1]!;
      const newStack = prev.slice(0, -1);
      setCurrentCursor(popped);
      cursorChangeRef.current = true;
      return newStack;
    });
  }, []);

  const refetch = useCallback(async () => {
    await fetchTILs(buildParams(filters, currentCursor));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchTILs, nonQueryKey, filters.query, currentCursor]);

  const deleteTIL = useCallback(async (id: number) => {
    await tilsAPI.delete(id);
    await fetchTILs(buildParams(filters, currentCursor));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchTILs, nonQueryKey, filters.query, currentCursor]);

  return {
    tils,
    hasMore,
    filterOptions,
    loading,
    error,
    refetch,
    goNext,
    goPrev,
    canGoPrev: cursorStack.length > 0,
    deleteTIL,
  };
}
