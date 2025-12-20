import { useState, useEffect, useCallback, useRef } from 'react';
import { POLLING_CONFIG } from '@/config/polling';

interface UseUserActivityReturn {
  /** Whether the user is currently idle (no activity for IDLE_THRESHOLD_MS) */
  isIdle: boolean;
  /** Manually mark the user as active (resets idle timer) */
  markActive: () => void;
}

/**
 * Hook that tracks user activity via DOM events.
 * Returns whether the user is idle (no activity for a threshold period).
 */
export function useUserActivity(): UseUserActivityReturn {
  const [isIdle, setIsIdle] = useState(false);
  const lastActivityRef = useRef(0);
  const checkIntervalRef = useRef<number | null>(null);

  const markActive = useCallback(() => {
    lastActivityRef.current = Date.now();
    if (isIdle) {
      setIsIdle(false);
    }
  }, [isIdle]);

  useEffect(() => {
    // Initialize last activity time on mount
    lastActivityRef.current = Date.now();

    // Set up activity listeners
    const handleActivity = () => {
      lastActivityRef.current = Date.now();
      if (isIdle) {
        setIsIdle(false);
      }
    };

    // Add listeners for all activity events
    for (const event of POLLING_CONFIG.ACTIVITY_EVENTS) {
      document.addEventListener(event, handleActivity, { passive: true });
    }

    // Periodically check if user has become idle
    checkIntervalRef.current = window.setInterval(() => {
      const elapsed = Date.now() - lastActivityRef.current;
      if (elapsed >= POLLING_CONFIG.IDLE_THRESHOLD_MS && !isIdle) {
        setIsIdle(true);
      }
    }, 5000); // Check every 5 seconds

    return () => {
      for (const event of POLLING_CONFIG.ACTIVITY_EVENTS) {
        document.removeEventListener(event, handleActivity);
      }
      if (checkIntervalRef.current !== null) {
        clearInterval(checkIntervalRef.current);
      }
    };
  }, [isIdle]);

  return { isIdle, markActive };
}
