import { useCallback, useRef, useEffect } from 'react';
import { useSmartPolling } from './useSmartPolling';
import { sessionsAPI } from '@/services/api';
import type { SessionListView } from '@/services/api';
import type { Session } from '@/types';
import type { PollingState } from '@/config/polling';

interface UseSessionsPollingReturn {
  /** Current sessions data */
  sessions: Session[];
  /** Current polling state: 'suspended' | 'passive' | 'active' */
  pollingState: PollingState;
  /** Manually trigger a refresh */
  refetch: () => Promise<void>;
  /** Whether a fetch is in progress */
  loading: boolean;
  /** Last error, if any */
  error: Error | null;
}

/**
 * Hook for polling sessions list with smart polling and ETag support.
 *
 * Uses visibility and activity detection to adjust polling frequency:
 * - suspended: Tab not visible, no polling
 * - passive: Tab visible but user idle, 60s intervals
 * - active: Tab visible and user active, 30s intervals
 *
 * Uses ETag headers to minimize bandwidth when data hasn't changed.
 *
 * @param view - Which sessions to show: 'owned' or 'shared'
 * @param enabled - Whether polling is enabled (default: true)
 */
export function useSessionsPolling(
  view: SessionListView = 'owned',
  enabled = true
): UseSessionsPollingReturn {
  // Track ETag for conditional requests
  const etagRef = useRef<string | null>(null);

  // Reset ETag when view changes (ETags are view-specific)
  useEffect(() => {
    etagRef.current = null;
  }, [view]);

  // Fetch function that handles ETag
  const fetchSessions = useCallback(async (): Promise<Session[] | null> => {
    const { data, etag } = await sessionsAPI.listWithETag(view, etagRef.current);

    // Update stored ETag
    if (etag) {
      etagRef.current = etag;
    }

    // data is null on 304 (no change)
    return data;
  }, [view]);

  const { data, state, refetch, loading, error } = useSmartPolling(fetchSessions, {
    enabled,
  });

  return {
    sessions: data ?? [],
    pollingState: state,
    refetch,
    loading,
    error,
  };
}
