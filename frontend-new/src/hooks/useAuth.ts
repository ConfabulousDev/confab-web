import { useQuery } from '@tanstack/react-query';
import { authAPI, AuthenticationError } from '@/services/api';

interface User {
  name: string;
  email: string;
  avatar_url: string;
}

interface UseAuthReturn {
  user: User | null;
  loading: boolean;
  error: string | null;
  isAuthenticated: boolean;
  refetch: () => void;
}

/**
 * Hook for managing authentication state with React Query
 */
export function useAuth(): UseAuthReturn {
  const { data: user, isLoading, error, refetch } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: authAPI.me,
    retry: false, // Don't retry on auth errors
    staleTime: 10 * 60 * 1000, // 10 minutes
  });

  return {
    user: user ?? null,
    loading: isLoading,
    error: error instanceof AuthenticationError
      ? null // Not authenticated is not an error
      : error instanceof Error
        ? error.message
        : null,
    isAuthenticated: user !== undefined && user !== null,
    refetch: () => { refetch(); },
  };
}
