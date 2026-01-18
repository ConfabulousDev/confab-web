import { useCallback, useRef, useEffect } from 'react';
import { useSmartPolling } from './useSmartPolling';
import { analyticsAPI } from '@/services/api';
import type { SessionAnalytics } from '@/schemas/api';
import type { PollingState } from '@/config/polling';

interface UseAnalyticsPollingReturn {
  /** Current analytics data */
  analytics: SessionAnalytics | null;
  /** Current polling state: 'suspended' | 'passive' | 'active' */
  pollingState: PollingState;
  /** Manually trigger a refresh */
  refetch: () => Promise<void>;
  /** Force a fresh fetch, bypassing 304 caching */
  forceRefetch: () => Promise<void>;
  /** Whether a fetch is in progress */
  loading: boolean;
  /** Last error, if any */
  error: Error | null;
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
    }

    const result = await analyticsAPI.get(sessionId, computedLinesRef.current);

    // Ignore stale response if session changed during fetch
    // This prevents race conditions when switching sessions quickly
    if (requestSessionId !== currentSessionIdRef.current) {
      return null;
    }

    if (result !== null) {
      // Update stored computed_lines for next poll
      computedLinesRef.current = result.computed_lines;
    }

    // result is null on 304 (no change)
    return result;
  }, [sessionId]);

  const { data, state, refetch, loading, error } = useSmartPolling(fetchAnalytics, {
    enabled,
    resetKey: sessionId, // Triggers refetch when switching sessions
  });

  // Force a fresh fetch by resetting the cache-busting state
  const forceRefetch = useCallback(async () => {
    computedLinesRef.current = 0;
    await refetch();
  }, [refetch]);

  return {
    analytics: data,
    pollingState: state,
    refetch,
    forceRefetch,
    loading,
    error,
  };
}
