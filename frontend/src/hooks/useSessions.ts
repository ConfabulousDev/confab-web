import { useQuery } from '@tanstack/react-query';
import { sessionsAPI, AuthenticationError } from '@/services/api';
import type { SessionListView } from '@/services/api';
import type { Session } from '@/types';

interface UseSessionsReturn {
  sessions: Session[];
  loading: boolean;
  error: string | null;
  refetch: () => Promise<unknown>;
}

/**
 * Hook for fetching sessions list with React Query
 * @param view - Which sessions to show: 'owned' (default) or 'shared'
 */
export function useSessions(view: SessionListView = 'owned'): UseSessionsReturn {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['sessions', view],
    queryFn: () => sessionsAPI.list(view),
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
