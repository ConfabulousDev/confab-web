import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';

/**
 * Watches for server error recovery (serverError transitions from true to false)
 * and invalidates all query caches except auth so data hooks refetch fresh data.
 */
export function useServerRecovery(serverError: boolean): void {
  const queryClient = useQueryClient();
  const prevServerError = useRef(serverError);

  useEffect(() => {
    const wasError = prevServerError.current;
    prevServerError.current = serverError;

    // Recovery: was in error state, now resolved
    if (wasError && !serverError) {
      queryClient.invalidateQueries({
        predicate: (query) => {
          const key = query.queryKey;
          // Skip the auth query — it just succeeded
          return !(Array.isArray(key) && key[0] === 'auth' && key[1] === 'me');
        },
      });
    }
  }, [serverError, queryClient]);
}
