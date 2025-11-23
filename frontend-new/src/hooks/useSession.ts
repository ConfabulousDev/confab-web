import { useQuery } from '@tanstack/react-query';
import { sessionsAPI, AuthenticationError } from '@/services/api';
import type { SessionDetail } from '@/types';
import { useNavigate } from 'react-router-dom';
import { useEffect } from 'react';

interface UseSessionReturn {
  session: SessionDetail | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

/**
 * Hook for fetching session data with React Query
 */
export function useSession(sessionId: string | undefined): UseSessionReturn {
  const navigate = useNavigate();

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['sessions', sessionId],
    queryFn: () => sessionsAPI.get(sessionId!),
    enabled: !!sessionId, // Only run if sessionId exists
  });

  // Redirect on auth error
  useEffect(() => {
    if (error instanceof AuthenticationError) {
      navigate('/');
    }
  }, [error, navigate]);

  return {
    session: data ?? null,
    loading: isLoading,
    error: error instanceof AuthenticationError
      ? null
      : error instanceof Error
        ? error.message
        : null,
    refetch: () => { refetch(); },
  };
}
