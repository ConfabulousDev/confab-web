import { useState, useCallback, useEffect, useRef } from 'react';
import { sessionsAPI } from '@/services/api';
import type { Session } from '@/types';
import type { SessionFilterOptions } from '@/schemas/api';
import type { SessionFilters } from './useSessionFilters';

interface UseSessionsFetchReturn {
  sessions: Session[];
  total: number;
  page: number;
  pageSize: number;
  filterOptions: SessionFilterOptions | null;
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
}

function buildParams(filters: SessionFilters): Record<string, string> {
  const params: Record<string, string> = {};
  if (filters.repos.length > 0) params.repo = filters.repos.join(',');
  if (filters.branches.length > 0) params.branch = filters.branches.join(',');
  if (filters.owners.length > 0) params.owner = filters.owners.join(',');
  if (filters.query) params.q = filters.query;
  if (filters.page > 1) params.page = String(filters.page);
  return params;
}

/**
 * Hook for fetching the paginated sessions list with server-side filtering.
 * Debounces search query changes by 300ms.
 */
export function useSessionsFetch(filters: SessionFilters): UseSessionsFetchReturn {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [filterOptions, setFilterOptions] = useState<SessionFilterOptions | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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
      setTotal(result.total);
      setPage(result.page);
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
    page: filters.page,
  });

  // Fetch immediately when non-query filters change
  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    fetchSessions(buildParams(filters));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nonQueryKey]);

  // Debounce query changes
  const queryRef = useRef(filters.query);
  useEffect(() => {
    // Skip if query hasn't actually changed (initial mount handled by nonQueryKey effect)
    if (queryRef.current === filters.query) return;
    queryRef.current = filters.query;

    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }
    debounceRef.current = setTimeout(() => {
      fetchSessions(buildParams(filters));
    }, 300);

    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.query]);

  const refetch = useCallback(async () => {
    await fetchSessions(buildParams(filters));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchSessions, nonQueryKey, filters.query]);

  return {
    sessions,
    total,
    page,
    pageSize,
    filterOptions,
    loading,
    error,
    refetch,
  };
}
