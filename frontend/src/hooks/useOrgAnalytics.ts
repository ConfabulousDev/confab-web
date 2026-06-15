import { orgAnalyticsAPI, type OrgAnalyticsParams } from '@/services/api';
import type { OrgAnalyticsResponse } from '@/schemas/api';
import { useApiData, type UseApiDataReturn } from './useApiData';

/**
 * Hook for fetching organization-level analytics with filter parameters.
 *
 * @param initialParams - Initial filter parameters
 */
export function useOrgAnalytics(
  initialParams: OrgAnalyticsParams,
): UseApiDataReturn<OrgAnalyticsResponse, OrgAnalyticsParams> {
  return useApiData(orgAnalyticsAPI.get, initialParams, 'Failed to fetch org analytics');
}
