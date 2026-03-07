import { useState, useEffect, useRef } from 'react';

interface UseAutoRetryOptions {
  /** Maximum number of auto-retry attempts before stopping. */
  maxAttempts: number;
  /** Initial delay in milliseconds before the first retry. */
  initialDelay: number;
  /** Maximum delay in milliseconds (caps exponential backoff). */
  maxDelay: number;
  /** Whether auto-retry is active. Set to false to pause/stop. */
  enabled: boolean;
}

interface UseAutoRetryReturn {
  /** Seconds remaining until the next auto-retry. */
  countdown: number;
  /** Current attempt number (1-based). 0 if not yet started. */
  attempt: number;
  /** True while the retry function is executing. */
  isRetrying: boolean;
  /** True when maxAttempts has been reached. Manual retry still works. */
  exhausted: boolean;
}

/**
 * Hook that periodically calls a retry function with exponential backoff.
 * The next countdown starts only after the current retry resolves (success or failure),
 * preventing overlapping requests from slow network timeouts.
 */
export function useAutoRetry(
  retryFn: () => Promise<unknown>,
  options: UseAutoRetryOptions,
): UseAutoRetryReturn {
  const { maxAttempts, initialDelay, maxDelay, enabled } = options;

  const [countdown, setCountdown] = useState(0);
  const [attempt, setAttempt] = useState(0);
  const [isRetrying, setIsRetrying] = useState(false);
  const [exhausted, setExhausted] = useState(false);

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const attemptRef = useRef(0);

  useEffect(() => {
    let cancelled = false;

    function clearTimer(): void {
      if (timerRef.current !== null) {
        clearInterval(timerRef.current);
        timerRef.current = null;
      }
    }

    function getDelay(attemptIndex: number): number {
      return Math.min(initialDelay * Math.pow(2, attemptIndex), maxDelay);
    }

    if (!enabled) {
      clearTimer();
      setCountdown(0);
      setAttempt(0);
      setIsRetrying(false);
      setExhausted(false);
      attemptRef.current = 0;
      return;
    }

    const startCountdown = (delayMs: number) => {
      let remaining = Math.ceil(delayMs / 1000);
      setCountdown(remaining);

      timerRef.current = setInterval(() => {
        remaining -= 1;
        if (remaining <= 0) {
          clearTimer();
          setCountdown(0);
          doRetry();
        } else {
          setCountdown(remaining);
        }
      }, 1000);
    };

    const doRetry = async () => {
      const currentAttempt = attemptRef.current + 1;
      attemptRef.current = currentAttempt;
      setAttempt(currentAttempt);
      setIsRetrying(true);

      try {
        await retryFn();
      } catch {
        // Don't schedule new timers if the effect was cleaned up during the await
        if (cancelled) return;
        setIsRetrying(false);
        if (currentAttempt >= maxAttempts) {
          setExhausted(true);
        } else {
          startCountdown(getDelay(currentAttempt));
        }
      }
    };

    startCountdown(getDelay(0));

    return () => {
      cancelled = true;
      clearTimer();
    };
  // Only re-run when enabled changes
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  return { countdown, attempt, isRetrying, exhausted };
}
