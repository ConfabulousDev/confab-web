import { useState, useCallback, useRef, useEffect } from 'react';
import { orgAnalyticsAPI, type OrgAnalyticsParams } from '@/services/api';
import type { OrgAnalyticsResponse } from '@/schemas/api';

interface UseOrgAnalyticsReturn {
  data: OrgAnalyticsResponse | null;
  loading: boolean;
  error: Error | null;
  refetch: (params: OrgAnalyticsParams) => Promise<void>;
}

export function useOrgAnalytics(initialParams: OrgAnalyticsParams): UseOrgAnalyticsReturn {
  const [data, setData] = useState<OrgAnalyticsResponse | null>(null);
  const [loading, setLoading] = useState(true);
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
    fetchData(initialParamsRef.current);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- only fetch once on mount

  return { data, loading, error, refetch: fetchData };
}
