import { useQuery } from '@tanstack/react-query';
import { sessionsAPI, AuthenticationError } from '@/services/api';
import type { Session } from '@/types';
import { useNavigate } from 'react-router-dom';
import { useEffect } from 'react';

interface UseSessionsReturn {
  sessions: Session[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

/**
 * Hook for fetching sessions list with React Query
 */
export function useSessions(): UseSessionsReturn {
  const navigate = useNavigate();

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['sessions'],
    queryFn: sessionsAPI.list,
  });

  // Redirect on auth error
  useEffect(() => {
    if (error instanceof AuthenticationError) {
      navigate('/');
    }
  }, [error, navigate]);

  return {
    sessions: data ?? [],
    loading: isLoading,
    error: error instanceof AuthenticationError
      ? null
      : error instanceof Error
        ? error.message
        : null,
    refetch: () => { refetch(); },
  };
}
