import { trendsAPI, type TrendsParams } from '@/services/api';
import type { TrendsResponse } from '@/schemas/api';
import { useApiData, type UseApiDataReturn } from './useApiData';

/**
 * Hook for fetching trends data with filter parameters.
 *
 * @param initialParams - Initial filter parameters
 */
export function useTrends(
  initialParams: TrendsParams = {},
): UseApiDataReturn<TrendsResponse, TrendsParams> {
  return useApiData(trendsAPI.get, initialParams, 'Failed to fetch trends');
}
