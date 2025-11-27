import { useQuery } from '@tanstack/react-query';
import { sessionsAPI, AuthenticationError } from '@/services/api';
import type { Session } from '@/types';

interface UseSessionsReturn {
  sessions: Session[];
  loading: boolean;
  error: string | null;
  refetch: () => Promise<unknown>;
}

/**
 * Hook for fetching sessions list with React Query
 * @param includeShared - Whether to include sessions shared with the user (default: false)
 */
export function useSessions(includeShared = false): UseSessionsReturn {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['sessions', includeShared],
    queryFn: () => sessionsAPI.list(includeShared),
  });

  // Auth errors are handled globally by the API client
  return {
    sessions: data ?? [],
    loading: isLoading,
    error: error instanceof AuthenticationError
      ? null
      : error instanceof Error
        ? error.message
        : null,
    refetch,
  };
}
