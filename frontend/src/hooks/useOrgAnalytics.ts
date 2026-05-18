import { useState, useCallback, useRef, useEffect } from 'react';
import { orgAnalyticsAPI, type OrgAnalyticsParams } from '@/services/api';
import type { OrgAnalyticsResponse } from '@/schemas/api';

interface UseOrgAnalyticsOptions {
  // When false, the hook skips its on-mount fetch. The caller must drive
  // the first request via `refetch` once it's ready. Useful when the page
  // can't decide the right params until a sibling resource (e.g. the org
  // repo list) has loaded — firing with the wrong default would render
  // wrong data and waste a request.
  enabled?: boolean;
}

interface UseOrgAnalyticsReturn {
  data: OrgAnalyticsResponse | null;
  loading: boolean;
  error: Error | null;
  refetch: (params: OrgAnalyticsParams) => Promise<void>;
}

export function useOrgAnalytics(
  initialParams: OrgAnalyticsParams,
  options: UseOrgAnalyticsOptions = {},
): UseOrgAnalyticsReturn {
  const enabled = options.enabled ?? true;
  const [data, setData] = useState<OrgAnalyticsResponse | null>(null);
  const [loading, setLoading] = useState(enabled);
  const [error, setError] = useState<Error | null>(null);
  const initialParamsRef = useRef(initialParams);

  const fetchData = useCallback(async (params: OrgAnalyticsParams) => {
    setLoading(true);
    setError(null);

    try {
      const response = await orgAnalyticsAPI.get(params);
      setData(response);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch org analytics'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;
    fetchData(initialParamsRef.current);
  }, [enabled]); // eslint-disable-line react-hooks/exhaustive-deps -- only fetch once on enable

  return { data, loading, error, refetch: fetchData };
}
