import { useState, useEffect } from 'react';

// ASCII spinner frames
const SPINNER_FRAMES = ['|', '/', '-', '\\'] as const;

/**
 * Hook to create an ASCII spinner animation
 * Returns a character that cycles through spinner frames when active
 */
export function useSpinner(active: boolean): string {
  const [frameIndex, setFrameIndex] = useState(0);

  useEffect(() => {
    if (!active) return;

    const interval = setInterval(() => {
      setFrameIndex((prev) => (prev + 1) % SPINNER_FRAMES.length);
    }, 100);

    return () => clearInterval(interval);
  }, [active]);

  return SPINNER_FRAMES[frameIndex % SPINNER_FRAMES.length] ?? '|';
}
