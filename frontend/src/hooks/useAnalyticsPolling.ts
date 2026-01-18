import { useCallback, useRef, useEffect } from 'react';
import { useSmartPolling } from './useSmartPolling';
import { analyticsAPI } from '@/services/api';
import type { SessionAnalytics } from '@/schemas/api';
import { POLLING_CONFIG, type PollingState } from '@/config/polling';

interface UseAnalyticsPollingReturn {
  /** Current analytics data */
  analytics: SessionAnalytics | null;
  /** Current polling state: 'suspended' | 'passive' | 'active' */
  pollingState: PollingState;
  /** Manually trigger a refresh */
  refetch: () => Promise<void>;
  /** Force a fresh fetch, bypassing 304 caching (use after triggering regeneration) */
  forceRefetch: () => Promise<void>;
  /** Whether a fetch is in progress */
  loading: boolean;
  /** Last error, if any */
  error: Error | null;
  /** Whether smart recap is currently being generated (fast polling active) */
  isSmartRecapGenerating: boolean;
}

/**
 * Hook for polling session analytics with smart polling.
 *
 * Uses visibility and activity detection to adjust polling frequency:
 * - suspended: Tab not visible, no polling
 * - passive: Tab visible but user idle, 60s intervals
 * - active: Tab visible and user active, 30s intervals
 *
 * Uses as_of_line parameter to minimize bandwidth when data hasn't changed.
 * Backend returns 304 Not Modified if client already has latest analytics.
 *
 * @param sessionId - The session to fetch analytics for
 * @param enabled - Whether polling is enabled (default: true)
 */
export function useAnalyticsPolling(
  sessionId: string,
  enabled = true
): UseAnalyticsPollingReturn {
  // Track computed_lines for conditional requests
  const computedLinesRef = useRef<number>(0);
  const lastFetchedSessionRef = useRef<string | null>(null);
  // Track current session ID to detect stale responses from in-flight requests
  const currentSessionIdRef = useRef(sessionId);
  // Track if smart recap is generating - skip as_of_line to get fresh data
  const isGeneratingRef = useRef(false);

  // Keep current session ref updated
  useEffect(() => {
    currentSessionIdRef.current = sessionId;
  }, [sessionId]);

  // Fetch function that handles conditional requests
  const fetchAnalytics = useCallback(async (): Promise<SessionAnalytics | null> => {
    // Capture the session ID for this request to detect stale responses
    const requestSessionId = sessionId;

    // Reset computed_lines when session changes
    if (lastFetchedSessionRef.current !== sessionId) {
      computedLinesRef.current = 0;
      lastFetchedSessionRef.current = sessionId;
      isGeneratingRef.current = false;
    }

    // Don't pass as_of_line while smart recap is generating
    // This ensures we get fresh data when generation completes
    const asOfLine = isGeneratingRef.current ? 0 : computedLinesRef.current;
    const result = await analyticsAPI.get(sessionId, asOfLine);

    // Ignore stale response if session changed during fetch
    // This prevents race conditions when switching sessions quickly
    if (requestSessionId !== currentSessionIdRef.current) {
      return null;
    }

    if (result !== null) {
      // Update stored computed_lines for next poll
      computedLinesRef.current = result.computed_lines;

      // Track generating state for next poll
      const smartRecap = result.cards?.smart_recap;
      isGeneratingRef.current = smartRecap != null && 'status' in smartRecap && smartRecap.status === 'generating';
    }

    // result is null on 304 (no change)
    return result;
  }, [sessionId]);

  // Use faster polling when smart recap is generating
  const getIntervalOverride = useCallback(
    (analytics: SessionAnalytics | null): number | null => {
      // Check ref first - it's set synchronously in forceRefetch before data updates
      // This ensures fast polling kicks in immediately after triggering regeneration
      if (isGeneratingRef.current) {
        return POLLING_CONFIG.GENERATING_INTERVAL_MS;
      }
      const smartRecap = analytics?.cards?.smart_recap;
      if (smartRecap && 'status' in smartRecap && smartRecap.status === 'generating') {
        return POLLING_CONFIG.GENERATING_INTERVAL_MS;
      }
      return null;
    },
    []
  );

  const { data, state, refetch, loading, error } = useSmartPolling(fetchAnalytics, {
    enabled,
    resetKey: sessionId, // Triggers refetch when switching sessions
    intervalOverride: getIntervalOverride,
  });

  // Force a fresh fetch by resetting the cache-busting state
  // Use this after triggering smart recap regeneration to ensure we get the "generating" status
  const forceRefetch = useCallback(async () => {
    computedLinesRef.current = 0;
    isGeneratingRef.current = true;
    await refetch();
  }, [refetch]);

  // Check if smart recap is generating for UI feedback
  const isSmartRecapGenerating =
    data?.cards?.smart_recap != null &&
    'status' in data.cards.smart_recap &&
    data.cards.smart_recap.status === 'generating';

  return {
    analytics: data,
    pollingState: state,
    refetch,
    forceRefetch,
    loading,
    error,
    isSmartRecapGenerating,
  };
}
