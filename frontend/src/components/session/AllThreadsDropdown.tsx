// we3k D10: "All subagents (n)" dropdown pinned at the right of the thread strip.
// Lists Main, then every thread in strip order with status, full label and
// details; a filter appears once there are more than 8 subagents. Jumps to any
// thread, and is the main navigation on narrow screens (D15).

import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { useDropdown } from '@/hooks';
import { CheckIcon, ChevronIcon } from '@/components/icons';
import { formatDuration } from '@/components/transcript/timelineFormat';
import type { TranscriptThreadRef } from '@/providers/types';
import { cx } from '@/utils/utils';
import ThreadStatusGlyph from './ThreadStatusGlyph';
import { THREAD_STATUS_LABEL, isNestedThread, parentLabel, threadMatchesQuery, threadStatus } from './threadDetails';
import styles from './ThreadDropdown.module.css';

/** More subagents than this get a filter input. */
const FILTER_THRESHOLD = 8;

const ListGlyph = (
  <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <path d="M2 3h8M2 6h8M2 9h8" />
  </svg>
);

interface AllThreadsDropdownProps {
  threads: TranscriptThreadRef[];
  /** null = Main. */
  activeThreadId: string | null;
  onSelect: (threadId: string | null) => void;
}

/** A dropdown row: Main (`thread` undefined) or a thread. */
interface Row {
  id: string | null;
  thread?: TranscriptThreadRef;
}

export default function AllThreadsDropdown({ threads, activeThreadId, onSelect }: AllThreadsDropdownProps) {
  const { isOpen, setIsOpen, containerRef } = useDropdown<HTMLDivElement>();
  const [query, setQuery] = useState('');
  const [highlight, setHighlight] = useState(0);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();

  const count = threads.length;
  const showFilter = count > FILTER_THRESHOLD;
  const rows: Row[] = [
    ...('main'.includes(query.trim().toLowerCase()) ? [{ id: null }] : []),
    ...threads.filter((t) => threadMatchesQuery(t, query)).map((t) => ({ id: t.id, thread: t })),
  ];
  const highlighted = Math.min(highlight, rows.length - 1);
  const optionId = (index: number) => `${listId}-option-${index}`;
  const activeDescendant = highlighted >= 0 ? optionId(highlighted) : undefined;

  // Focus moves into the popover on open: the filter when present, else the list.
  useEffect(() => {
    if (isOpen) (inputRef.current ?? listRef.current)?.focus();
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    listRef.current?.querySelector<HTMLElement>('[data-highlighted="true"]')?.scrollIntoView?.({ block: 'nearest' });
  }, [isOpen, highlighted]);

  function open() {
    setQuery('');
    setHighlight(Math.max(0, threads.findIndex((t) => t.id === activeThreadId) + 1));
    setIsOpen(true);
  }

  function close() {
    setIsOpen(false);
    buttonRef.current?.focus();
  }

  function select(row: Row) {
    close();
    onSelect(row.id);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const last = rows.length - 1;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        setHighlight(highlighted >= last ? 0 : highlighted + 1);
        break;
      case 'ArrowUp':
        event.preventDefault();
        setHighlight(highlighted <= 0 ? last : highlighted - 1);
        break;
      case 'Enter': {
        const row = rows[highlighted];
        if (row) {
          event.preventDefault();
          select(row);
        }
        break;
      }
      case 'Escape':
        event.preventDefault();
        close();
        break;
      case 'Tab':
        setIsOpen(false);
        break;
    }
  }

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        ref={buttonRef}
        type="button"
        className={cx(styles.allButton, isOpen && styles.allButtonOpen)}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-controls={isOpen ? listId : undefined}
        aria-label={`All subagents (${count})`}
        onClick={() => (isOpen ? setIsOpen(false) : open())}
      >
        <span className={styles.allLabel}>All subagents ({count})</span>
        <span className={styles.allCompact}>
          {ListGlyph}
          {count}
        </span>
        <span className={styles.caret}>{ChevronIcon}</span>
      </button>

      {isOpen && (
        <div className={cx(styles.popover, styles.listPopover)} onKeyDown={handleKeyDown}>
          {showFilter && (
            <input
              ref={inputRef}
              type="search"
              className={styles.filter}
              placeholder="Filter subagents"
              aria-label="Filter subagents"
              aria-controls={listId}
              aria-activedescendant={activeDescendant}
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setHighlight(0);
              }}
            />
          )}
          {rows.length === 0 ? (
            <div className={styles.empty}>No subagents match</div>
          ) : (
            <ul
              ref={listRef}
              id={listId}
              role="listbox"
              aria-label="All subagents"
              aria-activedescendant={activeDescendant}
              tabIndex={-1}
              className={styles.list}
            >
              {rows.map((row, index) => (
                <ThreadRow
                  key={row.id ?? '\u0000main'}
                  id={optionId(index)}
                  row={row}
                  threads={threads}
                  selected={row.id === activeThreadId}
                  highlighted={index === highlighted}
                  onSelect={() => select(row)}
                  onHighlight={() => setHighlight(index)}
                />
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}

interface ThreadRowProps {
  id: string;
  row: Row;
  threads: TranscriptThreadRef[];
  selected: boolean;
  highlighted: boolean;
  onSelect: () => void;
  onHighlight: () => void;
}

function ThreadRow({ id, row, threads, selected, highlighted, onSelect, onHighlight }: ThreadRowProps) {
  const { thread } = row;
  const nested = thread !== undefined && isNestedThread(thread);
  return (
    <li
      id={id}
      role="option"
      aria-selected={selected}
      data-highlighted={highlighted}
      className={cx(
        styles.row,
        nested && styles.rowNested,
        highlighted && styles.rowHighlighted,
        selected && styles.rowSelected,
      )}
      onClick={onSelect}
      onMouseMove={highlighted ? undefined : onHighlight}
    >
      <span className={styles.rowGlyph}>{thread && <ThreadStatusGlyph status={threadStatus(thread)} />}</span>
      <span className={styles.rowBody}>
        <span className={styles.rowLabel}>
          {thread ? thread.label : 'Main'}
          {thread && <span className="visually-hidden">, {THREAD_STATUS_LABEL[threadStatus(thread)]}</span>}
        </span>
        {thread && (thread.subtitle || thread.model || thread.durationMs !== undefined || nested) && (
          <span className={styles.rowMeta}>
            {thread.subtitle && <span>{thread.subtitle}</span>}
            {nested && <span>Launched by {parentLabel(thread, threads)}</span>}
            <span className={styles.rowMetaEnd}>
              {thread.model && <span>{thread.model}</span>}
              {thread.durationMs !== undefined && <span>{formatDuration(thread.durationMs)}</span>}
            </span>
          </span>
        )}
      </span>
      {selected && <span className={styles.rowCheck}>{CheckIcon}</span>}
    </li>
  );
}
