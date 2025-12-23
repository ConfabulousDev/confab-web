import { useState, useEffect, useRef } from 'react';
import { formatRelativeTime } from '@/utils/formatting';
import { useVisibility } from './useVisibility';

// Interval durations based on timestamp age
const VERY_RECENT_INTERVAL = 2000; // 2 seconds for timestamps < 5 minutes
const RECENT_INTERVAL = 5000; // 5 seconds for timestamps < 1 hour
const HOUR_INTERVAL = 60000; // 60 seconds for hour+ timestamps

/**
 * Calculate the appropriate update interval based on timestamp age.
 * - Less than 5 minutes: update every 2 seconds
 * - Less than 1 hour: update every 5 seconds
 * - 1 hour or more: update every 60 seconds
 */
function getUpdateInterval(dateStr: string): number {
  const date = new Date(dateStr.endsWith('Z') ? dateStr : `${dateStr}Z`);
  const now = new Date();
  const diffMs = Math.abs(now.getTime() - date.getTime());

  const FIVE_MINUTES = 5 * 60 * 1000;
  const ONE_HOUR = 60 * 60 * 1000;

  if (diffMs < FIVE_MINUTES) {
    return VERY_RECENT_INTERVAL;
  } else if (diffMs < ONE_HOUR) {
    return RECENT_INTERVAL;
  } else {
    return HOUR_INTERVAL;
  }
}

/**
 * Hook that returns a formatted relative time string that automatically updates.
 * Updates are paused when the tab is not visible.
 */
export function useRelativeTime(dateStr: string): string {
  const [, setTick] = useState(0);
  const isVisible = useVisibility();
  const intervalRef = useRef<number | null>(null);
  const currentIntervalDuration = useRef<number>(0);

  useEffect(() => {
    if (!isVisible) {
      // Reset duration tracking so interval resumes when visible again
      currentIntervalDuration.current = 0;
      return;
    }

    const setupInterval = () => {
      const interval = getUpdateInterval(dateStr);

      // Only reset if interval duration changed
      if (interval !== currentIntervalDuration.current) {
        if (intervalRef.current !== null) {
          clearInterval(intervalRef.current);
        }
        currentIntervalDuration.current = interval;
        intervalRef.current = window.setInterval(() => {
          setTick((t) => t + 1);
          // Check if we need to adjust the interval
          const newInterval = getUpdateInterval(dateStr);
          if (newInterval !== currentIntervalDuration.current) {
            setupInterval();
          }
        }, interval);
      }
    };

    setupInterval();

    return () => {
      if (intervalRef.current !== null) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [dateStr, isVisible]);

  return formatRelativeTime(dateStr);
}
