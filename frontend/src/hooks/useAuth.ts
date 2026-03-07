import { useQuery } from '@tanstack/react-query';
import { authAPI, AuthenticationError } from '@/services/api';
import type { User } from '@/schemas/api';

interface UseAuthReturn {
  user: User | null;
  loading: boolean;
  error: string | null;
  isAuthenticated: boolean;
  /** True when /me failed with a non-auth error (5xx, network) and no cached user exists. */
  serverError: boolean;
  refetch: () => Promise<unknown>;
}

/**
 * Hook for managing authentication state with React Query
 */
export function useAuth(): UseAuthReturn {
  const { data: user, isLoading, error, refetch } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: authAPI.me,
    retry: (failureCount, err) => {
      // Never retry 401s — pointless
      if (err instanceof AuthenticationError) return false;
      // Retry 5xx / network errors twice (covers brief OOM restarts)
      return failureCount < 2;
    },
    retryDelay: (attempt) => Math.min(1000 * Math.pow(2, attempt), 5000),
    staleTime: 10 * 60 * 1000, // 10 minutes
    refetchOnMount: false, // Don't refetch when new components mount
    refetchOnReconnect: false, // Don't refetch on reconnect
  });

  // Consider authenticated if we have cached user data, even during refetch
  const hasUser = user !== undefined && user !== null;

  // Server error = non-auth error with no cached user data
  const isServerError = !hasUser && error !== null && !(error instanceof AuthenticationError);

  // Auth errors (401) are expected, not surfaced. Only show unexpected errors.
  let errorMessage: string | null = null;
  if (error !== null && !(error instanceof AuthenticationError) && error instanceof Error) {
    errorMessage = error.message;
  }

  return {
    user: user ?? null,
    loading: isLoading,
    error: errorMessage,
    isAuthenticated: hasUser,
    serverError: isServerError,
    refetch,
  };
}
