import { useState, useEffect, useCallback } from 'react';
import { trendsAPI, type TrendsParams } from '@/services/api';
import type { TrendsResponse } from '@/schemas/api';

export interface UseTrendsReturn {
  /** Current trends data */
  data: TrendsResponse | null;
  /** Whether a fetch is in progress */
  loading: boolean;
  /** Last error, if any */
  error: Error | null;
  /** Manually trigger a refresh with optional new params */
  refetch: (params?: TrendsParams) => Promise<void>;
}

/**
 * Hook for fetching trends data with filter parameters.
 *
 * @param initialParams - Initial filter parameters
 */
export function useTrends(initialParams: TrendsParams = {}): UseTrendsReturn {
  const [data, setData] = useState<TrendsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [params, setParams] = useState<TrendsParams>(initialParams);

  const fetchTrends = useCallback(async (fetchParams: TrendsParams = params) => {
    setLoading(true);
    setError(null);

    try {
      const response = await trendsAPI.get(fetchParams);
      setData(response);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch trends'));
    } finally {
      setLoading(false);
    }
  }, [params]);

  const refetch = useCallback(async (newParams?: TrendsParams) => {
    if (newParams) {
      setParams(newParams);
    }
    await fetchTrends(newParams ?? params);
  }, [fetchTrends, params]);

  // Initial fetch
  useEffect(() => {
    fetchTrends(params);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return {
    data,
    loading,
    error,
    refetch,
  };
}
