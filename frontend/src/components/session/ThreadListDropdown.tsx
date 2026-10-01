// we3k D10 / jgk8: a listbox dropdown of transcript threads. Backs "All
// subagents (n)" at the right of the strip (Main first, plus a footer), and the
// subagent path's sibling segments and "Launched here (n)". Rows show status,
// full label and details; a filter appears once a list has more than 8 threads.

import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useDropdown } from '@/hooks';
import { CheckIcon, ChevronIcon } from '@/components/icons';
import { formatDuration } from '@/components/transcript/timelineFormat';
import type { TranscriptThreadRef } from '@/providers/types';
import { cx } from '@/utils/utils';
import ThreadStatusGlyph from './ThreadStatusGlyph';
import { THREAD_STATUS_LABEL, threadMatchesQuery, threadStatus } from './threadDetails';
import styles from './ThreadDropdown.module.css';

/** More threads than this get a filter input. */
const FILTER_THRESHOLD = 8;

interface ThreadListDropdownProps {
  threads: readonly TranscriptThreadRef[];
  /** Checked row; null = Main (or nothing when Main isn't listed). */
  activeThreadId: string | null;
  onSelect: (threadId: string | null) => void;
  /** The trigger's accessible name. */
  label: string;
  /** The list's accessible name; defaults to `label`. */
  listLabel?: string;
  /** Visible trigger content; defaults to `label`. */
  buttonContent?: ReactNode;
  /** List Main before the threads (All subagents). */
  includeMain?: boolean;
  /** Muted note under the list. */
  footer?: string;
  /** Known children per thread, shown as "launched N". */
  childCountOf?: (threadId: string) => number;
  /** Which trigger edge the popover lines up with. */
  align?: 'start' | 'end';
  triggerClassName?: string;
}

/** A dropdown row: Main (`thread` undefined) or a thread. */
interface Row {
  id: string | null;
  thread?: TranscriptThreadRef;
}

export default function ThreadListDropdown({
  threads,
  activeThreadId,
  onSelect,
  label,
  listLabel = label,
  buttonContent = label,
  includeMain = false,
  footer,
  childCountOf,
  align = 'end',
  triggerClassName,
}: ThreadListDropdownProps) {
  const { isOpen, setIsOpen, containerRef } = useDropdown<HTMLDivElement>();
  const [query, setQuery] = useState('');
  const [highlight, setHighlight] = useState(0);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();

  const showFilter = threads.length > FILTER_THRESHOLD;
  const rows: Row[] = [
    ...(includeMain && 'main'.includes(query.trim().toLowerCase()) ? [{ id: null }] : []),
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
    const activeIndex = threads.findIndex((t) => t.id === activeThreadId);
    const mainOffset = includeMain ? 1 : 0;
    setHighlight(activeIndex >= 0 ? activeIndex + mainOffset : 0);
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
        className={cx(styles.trigger, isOpen && styles.triggerOpen, triggerClassName)}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-controls={isOpen ? listId : undefined}
        aria-label={label}
        onClick={() => (isOpen ? setIsOpen(false) : open())}
      >
        {buttonContent}
        <span className={styles.caret} aria-hidden="true">
          {ChevronIcon}
        </span>
      </button>

      {isOpen && (
        <div
          className={cx(styles.popover, styles.listPopover, align === 'start' && styles.listPopoverStart)}
          onKeyDown={handleKeyDown}
        >
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
              aria-label={listLabel}
              aria-activedescendant={activeDescendant}
              tabIndex={-1}
              className={styles.list}
            >
              {rows.map((row, index) => (
                <ThreadRow
                  key={row.id ?? '\u0000main'}
                  id={optionId(index)}
                  row={row}
                  childCount={row.id !== null ? (childCountOf?.(row.id) ?? 0) : 0}
                  selected={row.id === activeThreadId}
                  highlighted={index === highlighted}
                  onSelect={() => select(row)}
                  onHighlight={() => setHighlight(index)}
                />
              ))}
            </ul>
          )}
          {footer && <p className={styles.footer}>{footer}</p>}
        </div>
      )}
    </div>
  );
}

interface ThreadRowProps {
  id: string;
  row: Row;
  childCount: number;
  selected: boolean;
  highlighted: boolean;
  onSelect: () => void;
  onHighlight: () => void;
}

function ThreadRow({ id, row, childCount, selected, highlighted, onSelect, onHighlight }: ThreadRowProps) {
  const { thread } = row;
  const launched = childCount > 0;
  return (
    <li
      id={id}
      role="option"
      aria-selected={selected}
      data-highlighted={highlighted}
      className={cx(styles.row, highlighted && styles.rowHighlighted, selected && styles.rowSelected)}
      onClick={onSelect}
      onMouseMove={highlighted ? undefined : onHighlight}
    >
      <span className={styles.rowGlyph}>{thread && <ThreadStatusGlyph status={threadStatus(thread)} />}</span>
      <span className={styles.rowBody}>
        <span className={styles.rowLabel}>
          {thread ? thread.label : 'Main'}
          {thread && <span className="visually-hidden">, {THREAD_STATUS_LABEL[threadStatus(thread)]}</span>}
        </span>
        {thread && (thread.subtitle || thread.model || thread.durationMs !== undefined || launched) && (
          <span className={styles.rowMeta}>
            {thread.subtitle && <span>{thread.subtitle}</span>}
            {launched && <span>launched {childCount}</span>}
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
