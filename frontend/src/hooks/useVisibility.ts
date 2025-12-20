import { useState, useEffect } from 'react';

/**
 * Hook that tracks whether the document/tab is visible.
 * Returns true when the tab is in the foreground.
 */
export function useVisibility(): boolean {
  const [isVisible, setIsVisible] = useState(() =>
    typeof document !== 'undefined' ? document.visibilityState === 'visible' : true
  );

  useEffect(() => {
    const handleVisibilityChange = () => {
      setIsVisible(document.visibilityState === 'visible');
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);

  return isVisible;
}
