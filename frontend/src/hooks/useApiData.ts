import { useState, useEffect, useLayoutEffect, useCallback, useRef } from 'react';

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
  const errorMessageRef = useRef(errorMessage);
  // Latest-value refs: updated after each commit, before effects and paint.
  useLayoutEffect(() => {
    fetchFnRef.current = fetchFn;
    errorMessageRef.current = errorMessage;
  });

  // Awaits the fetch and settles data/error/loading. Every state update happens
  // after the await, so the mount effect can call this directly.
  const runFetch = useCallback(async (fetchParams: P) => {
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
      setLoading(true);
      setError(null);
      await runFetch(newParams ?? params);
    },
    [runFetch, params],
  );

  // Initial fetch (once on mount). Initial state is already loading=true,
  // error=null, so no synchronous reset is needed here.
  useEffect(() => {
    runFetch(params);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only fetch once on mount
  }, []);

  return { data, loading, error, refetch };
}
