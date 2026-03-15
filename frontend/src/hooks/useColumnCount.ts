import { useState, useEffect } from 'react';

/** Breakpoints matching CardGrid: 700 (2 cols), 1100 (3 cols), 1400 (4 cols). */
function getColumnCount(): number {
  const w = window.innerWidth;
  if (w >= 1400) return 4;
  if (w >= 1100) return 3;
  if (w >= 700) return 2;
  return 1;
}

/** Distribute items round-robin across N columns (left-to-right reading order). */
export function distributeToColumns<T>(items: T[], columnCount: number): T[][] {
  const columns: T[][] = Array.from({ length: columnCount }, () => []);
  items.forEach((item, i) => columns[i % columnCount]!.push(item));
  return columns;
}

/** Returns a responsive column count (1–4) that updates on window resize. */
export function useColumnCount(): number {
  const [count, setCount] = useState(getColumnCount);

  useEffect(() => {
    function onResize() {
      setCount(getColumnCount());
    }
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  return count;
}
