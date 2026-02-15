import { useState, useCallback, useEffect, useRef } from 'react';
import { sessionsAPI } from '@/services/api';
import type { Session } from '@/types';

interface UseSessionsFetchReturn {
  /** Current sessions data */
  sessions: Session[];
  /** Manually trigger a refresh */
  refetch: () => Promise<void>;
  /** Whether a fetch is in progress */
  loading: boolean;
  /** Last error, if any */
  error: Error | null;
}

/**
 * Hook for fetching the unified sessions list (owned + shared).
 *
 * Fetches once on mount and provides a refetch function for manual refresh.
 * No polling — the user can trigger a refresh via the UI.
 *
 * @param enabled - Whether fetching is enabled (default: true). Used by TrendsPage.
 */
export function useSessionsFetch(enabled = true): UseSessionsFetchReturn {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const hasFetched = useRef(false);

  const fetchSessions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await sessionsAPI.list();
      setSessions(data);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch sessions'));
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch on mount (if enabled)
  useEffect(() => {
    if (!enabled) return;
    if (hasFetched.current) return;
    hasFetched.current = true;
    fetchSessions();
  }, [enabled, fetchSessions]);

  return {
    sessions,
    refetch: fetchSessions,
    loading,
    error,
  };
}
