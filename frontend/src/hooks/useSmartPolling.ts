import { useState, useEffect, useCallback, useRef } from 'react';
import { POLLING_CONFIG, type PollingState } from '@/config/polling';
import { useVisibility } from './useVisibility';
import { useUserActivity } from './useUserActivity';

interface UseSmartPollingOptions<T> {
  /** Function to merge new data with previous data. Default: replace */
  merge?: (prev: T | null, next: T) => T;
  /** Whether polling is enabled. Default: true */
  enabled?: boolean;
}

interface UseSmartPollingReturn<T> {
  /** Current data */
  data: T | null;
  /** Current polling state */
  state: PollingState;
  /** Manually trigger a fetch */
  refetch: () => Promise<void>;
  /** Whether a fetch is in progress */
  loading: boolean;
  /** Last error, if any */
  error: Error | null;
}

/**
 * Smart polling hook with visibility and activity awareness.
 *
 * Polling states:
 * - suspended: Tab not visible, no polling
 * - passive: Tab visible but user idle, slower polling (60s)
 * - active: Tab visible and user active, faster polling (30s)
 *
 * @param fetchFn - Function that fetches data. Return null to indicate "no change".
 * @param options - Configuration options
 */
export function useSmartPolling<T>(
  fetchFn: () => Promise<T | null>,
  options: UseSmartPollingOptions<T> = {}
): UseSmartPollingReturn<T> {
  const { merge, enabled = true } = options;

  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const isVisible = useVisibility();
  const { isIdle } = useUserActivity();

  // Derive polling state
  const state: PollingState = !isVisible
    ? 'suspended'
    : isIdle
      ? 'passive'
      : 'active';

  // Refs to avoid stale closures in timeout and prevent unnecessary effect triggers
  const fetchFnRef = useRef(fetchFn);
  const mergeRef = useRef(merge);
  const timeoutRef = useRef<number | null>(null);
  const isMountedRef = useRef(true);
  const isVisibleRef = useRef(isVisible);
  const isIdleRef = useRef(isIdle);
  const enabledRef = useRef(enabled);

  // Keep refs updated
  useEffect(() => {
    fetchFnRef.current = fetchFn;
    mergeRef.current = merge;
  }, [fetchFn, merge]);

  // Keep state refs updated (separate effect to avoid unnecessary triggers)
  useEffect(() => {
    isVisibleRef.current = isVisible;
    isIdleRef.current = isIdle;
    enabledRef.current = enabled;
  }, [isVisible, isIdle, enabled]);

  // Cleanup on unmount
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  const doFetch = useCallback(async () => {
    if (!isMountedRef.current) return;

    setLoading(true);
    setError(null);

    try {
      const result = await fetchFnRef.current();

      if (!isMountedRef.current) return;

      if (result !== null) {
        setData((prev) => {
          if (mergeRef.current) {
            return mergeRef.current(prev, result);
          }
          return result;
        });
      }
      // result === null means "no change", keep previous data
    } catch (err) {
      if (!isMountedRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (isMountedRef.current) {
        setLoading(false);
      }
    }
  }, []);

  // scheduleNext uses refs to avoid being recreated when state changes
  const scheduleNext = useCallback(() => {
    if (timeoutRef.current !== null) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }

    // Don't schedule if not visible or disabled
    if (!isVisibleRef.current || !enabledRef.current) return;

    const interval = isIdleRef.current
      ? POLLING_CONFIG.PASSIVE_INTERVAL_MS
      : POLLING_CONFIG.ACTIVE_INTERVAL_MS;

    timeoutRef.current = window.setTimeout(() => {
      doFetch().finally(() => {
        if (isMountedRef.current) {
          scheduleNext();
        }
      });
    }, interval);
  }, [doFetch]);

  // Handle visibility and enabled changes - fetch immediately when becoming visible
  useEffect(() => {
    if (!enabled) {
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
      return;
    }

    // Fetch immediately when becoming visible
    if (isVisible) {
      doFetch().finally(() => {
        if (isMountedRef.current) {
          scheduleNext();
        }
      });
    } else {
      // Clear timeout when hidden
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
    }

    return () => {
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
    };
  }, [isVisible, enabled, doFetch, scheduleNext]);

  // Reschedule when idle state changes (to adjust interval without fetching)
  useEffect(() => {
    // Only reschedule if visible and enabled - don't trigger a fetch
    // Use refs for the check since we only want to react to isIdle changes
    if (isVisibleRef.current && enabledRef.current) {
      scheduleNext();
    }
  }, [isIdle, scheduleNext]);

  const refetch = useCallback(async () => {
    await doFetch();
  }, [doFetch]);

  return { data, state, refetch, loading, error };
}
