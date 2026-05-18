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
  // Mirror the latest render's params so the on-enable auto-fire uses *current*
  // values, not the captured-at-mount values. OrgPage mounts with `repos: []`
  // and only fills it in after `/org/repos` lands + auto-select runs; capturing
  // at mount would issue the first request with stale `repos: []`, collapsing
  // `providers_present` to no-repo sessions only.
  const latestParamsRef = useRef(initialParams);
  useEffect(() => {
    latestParamsRef.current = initialParams;
  }, [initialParams]);

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
    fetchData(latestParamsRef.current);
  }, [enabled, fetchData]);

  return { data, loading, error, refetch: fetchData };
}
