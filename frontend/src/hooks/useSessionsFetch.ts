import { useState, useCallback, useEffect, useRef } from 'react';
import { sessionsAPI } from '@/services/api';
import type { Session } from '@/types';
import type { SessionFilterOptions } from '@/schemas/api';
import type { SessionFilters } from './useSessionFilters';

interface UseSessionsFetchReturn {
  sessions: Session[];
  hasMore: boolean;
  pageSize: number;
  filterOptions: SessionFilterOptions | null;
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
  goNext: () => void;
  goPrev: () => void;
  canGoPrev: boolean;
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
 * Hook for fetching the paginated sessions list with server-side filtering.
 * Uses cursor-based pagination with an in-memory cursor stack.
 * Debounces search query changes by 300ms.
 */
export function useSessionsFetch(filters: SessionFilters): UseSessionsFetchReturn {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState('');
  const [pageSize, setPageSize] = useState(50);
  const [filterOptions, setFilterOptions] = useState<SessionFilterOptions | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Cursor stack: previous cursors for "go back" navigation
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [currentCursor, setCurrentCursor] = useState('');

  const fetchSessions = useCallback(async (params: Record<string, string>) => {
    // Cancel any in-flight request
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);
    try {
      const result = await sessionsAPI.list(Object.keys(params).length > 0 ? params : undefined);
      if (controller.signal.aborted) return;
      setSessions(result.sessions);
      setHasMore(result.has_more);
      setNextCursor(result.next_cursor || '');
      setPageSize(result.page_size);
      setFilterOptions(result.filter_options);
    } catch (err) {
      if (controller.signal.aborted) return;
      setError(err instanceof Error ? err : new Error('Failed to fetch sessions'));
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
      }
    }
  }, []);

  // Serialize filter state (excluding query for debounce)
  const nonQueryKey = JSON.stringify({
    repos: filters.repos,
    branches: filters.branches,
    owners: filters.owners,
  });

  // Reset cursor when filters change
  const prevNonQueryKeyRef = useRef(nonQueryKey);
  const prevQueryRef = useRef(filters.query);

  // Fetch immediately when non-query filters change
  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    // Reset cursor if filters changed (not on initial mount)
    if (prevNonQueryKeyRef.current !== nonQueryKey) {
      prevNonQueryKeyRef.current = nonQueryKey;
      setCursorStack([]);
      setCurrentCursor('');
      fetchSessions(buildParams(filters, ''));
    } else {
      fetchSessions(buildParams(filters, currentCursor));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nonQueryKey]);

  // Debounce query changes
  useEffect(() => {
    // Skip if query hasn't actually changed (initial mount handled by nonQueryKey effect)
    if (prevQueryRef.current === filters.query) return;
    prevQueryRef.current = filters.query;

    // Reset cursor on query change
    setCursorStack([]);
    setCurrentCursor('');

    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }
    debounceRef.current = setTimeout(() => {
      fetchSessions(buildParams(filters, ''));
    }, 300);

    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.query]);

  // Re-fetch when cursor changes (from goNext/goPrev)
  const cursorChangeRef = useRef(false);
  useEffect(() => {
    if (!cursorChangeRef.current) return;
    cursorChangeRef.current = false;
    fetchSessions(buildParams(filters, currentCursor));
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
    await fetchSessions(buildParams(filters, currentCursor));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchSessions, nonQueryKey, filters.query, currentCursor]);

  return {
    sessions,
    hasMore,
    pageSize,
    filterOptions,
    loading,
    error,
    refetch,
    goNext,
    goPrev,
    canGoPrev: cursorStack.length > 0,
  };
}
