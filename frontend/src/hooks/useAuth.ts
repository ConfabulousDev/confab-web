import { useQuery } from '@tanstack/react-query';
import { authAPI, AuthenticationError } from '@/services/api';
import type { User } from '@/schemas/api';

interface UseAuthReturn {
  user: User | null;
  loading: boolean;
  error: string | null;
  isAuthenticated: boolean;
  refetch: () => Promise<unknown>;
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
    refetchOnMount: false, // Don't refetch when new components mount
    refetchOnReconnect: false, // Don't refetch on reconnect
  });

  // Consider authenticated if we have cached user data, even during refetch
  const hasUser = user !== undefined && user !== null;

  return {
    user: user ?? null,
    loading: isLoading, // Only true on initial load, not refetches
    error: error instanceof AuthenticationError
      ? null // Not authenticated is not an error
      : error instanceof Error
        ? error.message
        : null,
    isAuthenticated: hasUser,
    refetch,
  };
}
