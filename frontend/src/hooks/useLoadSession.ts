import { useState, useEffect, useCallback } from 'react';
import type { SessionDetail } from '@/types';
import { type SessionErrorType, statusToErrorType, getErrorMessage } from '@/utils/sessionErrors';

export type { SessionErrorType } from '@/utils/sessionErrors';

/** Type guard for objects with a status property */
function hasStatusProperty(err: unknown): err is { status: number } {
  return err !== null && typeof err === 'object' && 'status' in err && typeof err.status === 'number';
}

interface UseLoadSessionResult {
  session: SessionDetail | null;
  setSession: React.Dispatch<React.SetStateAction<SessionDetail | null>>;
  loading: boolean;
  error: string;
  errorType: SessionErrorType;
  setError: (error: string, type?: SessionErrorType) => void;
  clearError: () => void;
}

interface UseLoadSessionOptions {
  fetchSession: () => Promise<SessionDetail>;
  deps?: unknown[];
}

/**
 * Hook for loading session data with consistent state management.
 * Provides session, loading, and error state with typed error categories.
 */
export function useLoadSession({
  fetchSession,
  deps = [],
}: UseLoadSessionOptions): UseLoadSessionResult {
  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setErrorState] = useState('');
  const [errorType, setErrorType] = useState<SessionErrorType>(null);

  const setError = useCallback((message: string, type: SessionErrorType = 'general') => {
    setErrorState(message);
    setErrorType(type);
  }, []);

  const clearError = useCallback(() => {
    setErrorState('');
    setErrorType(null);
  }, []);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setLoading(true);
      clearError();

      try {
        const data = await fetchSession();
        if (!cancelled) {
          setSession(data);
        }
      } catch (err) {
        if (cancelled) return;

        // Handle specific error types - check for status property
        if (err instanceof Response || hasStatusProperty(err)) {
          const errorType = statusToErrorType(err.status);
          setError(getErrorMessage(errorType), errorType);
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load session', 'general');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    load();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return {
    session,
    setSession,
    loading,
    error,
    errorType,
    setError,
    clearError,
  };
}
