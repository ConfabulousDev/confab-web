import { useState, useRef, useEffect, useCallback } from 'react';

interface UseDropdownReturn<T extends HTMLElement> {
  isOpen: boolean;
  setIsOpen: (open: boolean) => void;
  toggle: () => void;
  containerRef: React.RefObject<T | null>;
}

/**
 * Hook for managing dropdown state with click-outside and escape key handling
 */
export function useDropdown<T extends HTMLElement = HTMLDivElement>(initialOpen = false): UseDropdownReturn<T> {
  const [isOpen, setIsOpen] = useState(initialOpen);
  const containerRef = useRef<T>(null);

  const toggle = useCallback(() => {
    setIsOpen((prev) => !prev);
  }, []);

  // Close dropdown when clicking outside
  useEffect(() => {
    if (!isOpen) return;

    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        event.target instanceof Node &&
        !containerRef.current.contains(event.target)
      ) {
        setIsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen]);

  // Close on escape key
  useEffect(() => {
    if (!isOpen) return;

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    }

    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen]);

  return { isOpen, setIsOpen, toggle, containerRef };
}
