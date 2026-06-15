import { useState, useEffect, useCallback, useRef } from 'react';

export interface UseApiDataReturn<T, P> {
  /** Latest fetched data, or null before the first successful load. */
  data: T | null;
  /** Whether a fetch is currently in progress. */
  loading: boolean;
  /** Last error, if any. */
  error: Error | null;
  /** Manually refetch. With no args, re-uses the last-used params. */
  refetch: (params?: P) => Promise<void>;
}

/**
 * Generic data-fetching hook: owns the loading/error/data state machine and a
 * manual `refetch`, fetching once on mount with `initialParams` (73q9).
 *
 * Params are tracked in state so a no-arg `refetch()` re-uses the last params
 * and `refetch(newParams)` both fetches with and remembers them. There is no
 * auto-refetch when `initialParams` changes — refreshing is always explicit via
 * `refetch`, matching the hooks this generalizes.
 *
 * `fetchFn` is read through a ref so passing a fresh closure each render doesn't
 * destabilize `refetch` or trigger refetches.
 */
export function useApiData<T, P>(
  fetchFn: (params: P) => Promise<T>,
  initialParams: P,
  errorMessage: string,
): UseApiDataReturn<T, P> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [params, setParams] = useState<P>(initialParams);

  const fetchFnRef = useRef(fetchFn);
  fetchFnRef.current = fetchFn;
  const errorMessageRef = useRef(errorMessage);
  errorMessageRef.current = errorMessage;

  const fetchData = useCallback(async (fetchParams: P) => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetchFnRef.current(fetchParams);
      setData(response);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(errorMessageRef.current));
    } finally {
      setLoading(false);
    }
  }, []);

  const refetch = useCallback(
    async (newParams?: P) => {
      if (newParams !== undefined) {
        setParams(newParams);
      }
      await fetchData(newParams ?? params);
    },
    [fetchData, params],
  );

  // Initial fetch (once on mount).
  useEffect(() => {
    fetchData(params);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only fetch once on mount
  }, []);

  return { data, loading, error, refetch };
}
