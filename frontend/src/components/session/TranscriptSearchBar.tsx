import { useEffect, type RefObject } from 'react';
import styles from './TranscriptSearchBar.module.css';

interface TranscriptSearchBarProps {
  query: string;
  onQueryChange: (query: string) => void;
  /** 1-based current match number */
  currentMatch: number;
  totalMatches: number;
  onNext: () => void;
  onPrev: () => void;
  onClose: () => void;
  inputRef: RefObject<HTMLInputElement | null>;
}

function TranscriptSearchBar({
  query,
  onQueryChange,
  currentMatch,
  totalMatches,
  onNext,
  onPrev,
  onClose,
  inputRef,
}: TranscriptSearchBarProps) {
  // Autofocus on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, [inputRef]);

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    } else if (e.key === 'Enter' && e.shiftKey) {
      e.preventDefault();
      onPrev();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      onNext();
    }
  }

  const hasMatches = totalMatches > 0;
  const matchText = query.trim()
    ? `${hasMatches ? currentMatch : 0} of ${totalMatches}`
    : '';

  return (
    <div className={styles.searchBar}>
      <input
        ref={inputRef}
        className={styles.input}
        type="text"
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Search transcript..."
        aria-label="Search transcript"
      />
      {matchText && <span className={styles.matchCount}>{matchText}</span>}
      <button
        className={styles.btn}
        onClick={onPrev}
        disabled={!hasMatches}
        title="Previous match (Shift+Enter)"
        aria-label="Previous match"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <polyline points="18 15 12 9 6 15" />
        </svg>
      </button>
      <button
        className={styles.btn}
        onClick={onNext}
        disabled={!hasMatches}
        title="Next match (Enter)"
        aria-label="Next match"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>
      <button
        className={styles.btn}
        onClick={onClose}
        title="Close search (Escape)"
        aria-label="Close search"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <line x1="18" y1="6" x2="6" y2="18" />
          <line x1="6" y1="6" x2="18" y2="18" />
        </svg>
      </button>
    </div>
  );
}

export default TranscriptSearchBar;
