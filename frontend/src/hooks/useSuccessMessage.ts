import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

interface UseSuccessMessageOptions {
  /** Param name to check in URL (default: 'success') */
  paramName?: string;
  /** Duration before starting fade (default: 4500ms) */
  fadeDuration?: number;
  /** Total duration before clearing message (default: 5000ms) */
  clearDuration?: number;
  /** Skip URL param checking (default: false) */
  skipUrlParams?: boolean;
}

interface UseSuccessMessageReturn {
  /** Current success message */
  message: string;
  /** Whether message is currently fading out */
  fading: boolean;
  /** Set a success message programmatically */
  setMessage: (msg: string) => void;
  /** Clear the message immediately */
  clearMessage: () => void;
}

/**
 * Hook for managing success messages with auto-dismiss and optional URL param support
 */
export function useSuccessMessage(
  options: UseSuccessMessageOptions = {}
): UseSuccessMessageReturn {
  const {
    paramName = 'success',
    fadeDuration = 4500,
    clearDuration = 5000,
    skipUrlParams = false,
  } = options;
  const [searchParams, setSearchParams] = useSearchParams();
  const [message, setMessageState] = useState('');
  const [fading, setFading] = useState(false);

  const clearMessage = useCallback(() => {
    setMessageState('');
    setFading(false);
  }, []);

  const setMessage = useCallback(
    (msg: string) => {
      setMessageState(msg);
      setFading(false);

      // Start fade out before clearing
      setTimeout(() => setFading(true), fadeDuration);
      setTimeout(() => {
        setMessageState('');
        setFading(false);
      }, clearDuration);
    },
    [fadeDuration, clearDuration]
  );

  // Check for success message from URL params on mount
  useEffect(() => {
    if (skipUrlParams) return;

    const successParam = searchParams.get(paramName);
    if (successParam) {
      setMessage(successParam);
      // Remove the success param from URL
      searchParams.delete(paramName);
      setSearchParams(searchParams, { replace: true });
    }
    // Only run on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { message, fading, setMessage, clearMessage };
}
