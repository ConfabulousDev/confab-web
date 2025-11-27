import { useState, useCallback } from 'react';

/**
 * Hook for managing a Set of toggled items (e.g., expanded/collapsed states)
 */
export function useToggleSet<T>() {
  const [items, setItems] = useState<Set<T>>(new Set());

  const toggle = useCallback((item: T) => {
    setItems((prev) => {
      const next = new Set(prev);
      if (next.has(item)) {
        next.delete(item);
      } else {
        next.add(item);
      }
      return next;
    });
  }, []);

  const has = useCallback((item: T) => items.has(item), [items]);

  return { items, toggle, has };
}
