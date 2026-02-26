import { useState, useMemo, useCallback, useRef, useEffect } from 'react';
import type { TranscriptLine } from '@/types';
import { parseMessage, extractTextContent } from '@/services/messageParser';

export interface TranscriptSearchResult {
  isOpen: boolean;
  query: string;
  matches: number[];
  currentMatchIndex: number;
  /** The filteredIndex of the currently active match, or null */
  currentMatchFilteredIndex: number | null;
  open: () => void;
  close: () => void;
  setQuery: (query: string) => void;
  goToNextMatch: () => void;
  goToPreviousMatch: () => void;
  inputRef: React.RefObject<HTMLInputElement | null>;
}

const DEBOUNCE_MS = 150;
const EMPTY_MATCHES: number[] = [];

/**
 * Hook for searching transcript messages with debounced query and match navigation.
 * Builds a lowercased search index from all message content (text, thinking, tool_use, tool_result).
 */
export function useTranscriptSearch(messages: TranscriptLine[]): TranscriptSearchResult {
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQueryState] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [currentMatchIndex, setCurrentMatchIndex] = useState(0);

  const inputRef = useRef<HTMLInputElement | null>(null);
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Build search index: lowercased text for each filtered message
  const searchIndex = useMemo(() => {
    return messages.map((msg) => {
      const parsed = parseMessage(msg);
      return extractTextContent(parsed.content).toLowerCase();
    });
  }, [messages]);

  // Compute matches from searchIndex + debouncedQuery (auto-recomputes on filter change)
  const matches = useMemo(() => {
    if (!debouncedQuery.trim()) return EMPTY_MATCHES;
    const needle = debouncedQuery.toLowerCase();
    const result: number[] = [];
    for (let i = 0; i < searchIndex.length; i++) {
      if (searchIndex[i]?.includes(needle)) {
        result.push(i);
      }
    }
    return result;
  }, [searchIndex, debouncedQuery]);

  // Reset currentMatchIndex when matches change.
  // setState during render is the React-recommended way to adjust state based on
  // derived values without an extra render cycle (see React docs: "Adjusting state
  // when a prop changes").
  const [prevMatches, setPrevMatches] = useState(matches);
  if (prevMatches !== matches) {
    setPrevMatches(matches);
    setCurrentMatchIndex(0);
  }

  const setQuery = useCallback((newQuery: string) => {
    setQueryState(newQuery);
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }
    debounceTimerRef.current = setTimeout(() => {
      setDebouncedQuery(newQuery);
    }, DEBOUNCE_MS);
  }, []);

  const open = useCallback(() => {
    setIsOpen((wasOpen) => {
      if (wasOpen) {
        // Already open — select all text in input
        inputRef.current?.focus();
        inputRef.current?.select();
      }
      return true;
    });
  }, []);

  const close = useCallback(() => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }
    setIsOpen(false);
    setQueryState('');
    setDebouncedQuery('');
    setCurrentMatchIndex(0);
  }, []);

  const goToNextMatch = useCallback(() => {
    setCurrentMatchIndex((prev) => {
      if (matches.length === 0) return prev;
      return (prev + 1) % matches.length;
    });
  }, [matches.length]);

  const goToPreviousMatch = useCallback(() => {
    setCurrentMatchIndex((prev) => {
      if (matches.length === 0) return prev;
      return (prev - 1 + matches.length) % matches.length;
    });
  }, [matches.length]);

  const currentMatchFilteredIndex =
    matches.length > 0 ? matches[currentMatchIndex] ?? null : null;

  // Cleanup debounce timer on unmount
  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, []);

  return {
    isOpen,
    query,
    matches,
    currentMatchIndex,
    currentMatchFilteredIndex,
    open,
    close,
    setQuery,
    goToNextMatch,
    goToPreviousMatch,
    inputRef,
  };
}
