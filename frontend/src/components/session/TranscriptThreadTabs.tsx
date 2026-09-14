// we3k: thread switcher under the Transcript tab (redesign of et0r's strip).
//
// `Main` is pinned outside a horizontal scroller of status-aware subagent
// chips. Overflow shows as edge fades and step buttons, never a native
// scrollbar; "All subagents" jumps anywhere. Clicking the already-active
// subagent chip opens its menu (Go to parent, Copy link). WAI-ARIA tabs with
// manual activation, so arrowing past chips never fetches their transcripts.
// Provider-agnostic: it only knows `TranscriptThreadRef`s.

import { useEffect, useRef, useState, type KeyboardEvent, type MouseEvent, type RefObject } from 'react';
import { useCopyToClipboard, useDropdown } from '@/hooks';
import { ChevronIcon } from '@/components/icons';
import Tooltip from '@/components/Tooltip';
import { formatDuration } from '@/components/transcript/timelineFormat';
import type { TranscriptThreadRef } from '@/providers/types';
import { cx } from '@/utils/utils';
import AllThreadsDropdown from './AllThreadsDropdown';
import ThreadStatusGlyph from './ThreadStatusGlyph';
import { THREAD_STATUS_LABEL, isNestedThread, parentLabel, threadStatus } from './threadDetails';
import dropdownStyles from './ThreadDropdown.module.css';
import styles from './TranscriptThreadTabs.module.css';

const MAIN_KEY = '\u0000main';
/** Step buttons scroll by this share of the visible strip. */
const STEP_RATIO = 0.8;
const MENU_MIN_WIDTH_PX = 220;
const COPIED_MS = 800;
/** Line-mode wheel deltas (Firefox) are converted at roughly one line of text. */
const WHEEL_LINE_PX = 16;

const NestedGlyph = (
  <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round">
    <path d="M2.5 1.5v3.25a1.5 1.5 0 0 0 1.5 1.5h4M6.5 4.75l1.5 1.5-1.5 1.5" />
  </svg>
);

interface TranscriptThreadTabsProps {
  threads: TranscriptThreadRef[];
  /** null = Main. */
  activeThreadId: string | null;
  /** For the subagent deep link the chip menu copies. */
  sessionId: string;
  /** Switch threads; `targetId` is the row to land on (Go to parent → the launch row). */
  onSelect: (threadId: string | null, targetId?: string) => void;
}

function prefersReducedMotion(): boolean {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches ?? false;
}

function scrollBehavior(): ScrollBehavior {
  return prefersReducedMotion() ? 'auto' : 'smooth';
}

function fadeClassFor(overflow: { start: boolean; end: boolean }): string | undefined {
  if (overflow.start && overflow.end) return styles.fadeBoth;
  if (overflow.start) return styles.fadeStart;
  if (overflow.end) return styles.fadeEnd;
  return undefined;
}

/** Whether a horizontal scroller hides content at its start / end; kept current on scroll, resize and content change. */
function useHorizontalOverflow(scrollerRef: RefObject<HTMLDivElement | null>, content: unknown) {
  const [overflow, setOverflow] = useState({ start: false, end: false });

  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const scroller = el;
    function update() {
      const start = scroller.scrollLeft > 1;
      const end = scroller.scrollLeft + scroller.clientWidth < scroller.scrollWidth - 1;
      setOverflow((prev) => (prev.start === start && prev.end === end ? prev : { start, end }));
    }
    const frame = requestAnimationFrame(update);
    scroller.addEventListener('scroll', update, { passive: true });
    const observer = new ResizeObserver(update);
    observer.observe(scroller);
    for (const child of Array.from(scroller.children)) observer.observe(child);
    return () => {
      cancelAnimationFrame(frame);
      scroller.removeEventListener('scroll', update);
      observer.disconnect();
    };
  }, [scrollerRef, content]);

  return overflow;
}

/** A vertical mouse wheel over the scroller scrolls it sideways (until an edge, then the page scrolls). */
function useWheelScrollsSideways(scrollerRef: RefObject<HTMLDivElement | null>) {
  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const scroller = el;
    function handleWheel(event: WheelEvent) {
      if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
      const max = scroller.scrollWidth - scroller.clientWidth;
      if (max <= 0) return;
      const delta = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? event.deltaY * WHEEL_LINE_PX : event.deltaY;
      const next = Math.max(0, Math.min(max, scroller.scrollLeft + delta));
      if (next === scroller.scrollLeft) return;
      event.preventDefault();
      scroller.scrollLeft = next;
    }
    scroller.addEventListener('wheel', handleWheel, { passive: false });
    return () => scroller.removeEventListener('wheel', handleWheel);
  }, [scrollerRef]);
}

function ChipDetails({ thread, threads }: { thread: TranscriptThreadRef; threads: TranscriptThreadRef[] }) {
  return (
    <div className={styles.details}>
      <div className={styles.detailsTitle}>{thread.label}</div>
      <dl className={styles.detailsList}>
        <dt>Status</dt>
        <dd>{THREAD_STATUS_LABEL[threadStatus(thread)]}</dd>
        {thread.subtitle && (
          <>
            <dt>Type</dt>
            <dd>{thread.subtitle}</dd>
          </>
        )}
        {thread.model && (
          <>
            <dt>Model</dt>
            <dd>{thread.model}</dd>
          </>
        )}
        {thread.durationMs !== undefined && (
          <>
            <dt>Duration</dt>
            <dd>{formatDuration(thread.durationMs)}</dd>
          </>
        )}
      </dl>
      {isNestedThread(thread) && <div className={styles.detailsNote}>Launched by {parentLabel(thread, threads)}</div>}
    </div>
  );
}

export default function TranscriptThreadTabs({ threads, activeThreadId, sessionId, onSelect }: TranscriptThreadTabsProps) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const chipRefs = useRef(new Map<string, HTMLButtonElement>());
  const [focusKey, setFocusKey] = useState<string | null>(null);
  const { isOpen: menuIsOpen, setIsOpen: setMenuOpen, containerRef: menuContainerRef } = useDropdown<HTMLDivElement>();
  const [menuAnchor, setMenuAnchor] = useState<{ threadId: string; left: number } | null>(null);
  const { copy, copied } = useCopyToClipboard({ messageDuration: COPIED_MS });
  const overflow = useHorizontalOverflow(scrollerRef, threads);
  useWheelScrollsSideways(scrollerRef);

  const keys = [MAIN_KEY, ...threads.map((t) => t.id)];
  const activeKey = activeThreadId !== null && keys.includes(activeThreadId) ? activeThreadId : MAIN_KEY;
  const rovingKey = focusKey !== null && keys.includes(focusKey) ? focusKey : activeKey;
  const activeThread = threads.find((t) => t.id === activeThreadId);
  // The menu belongs to the chip it was opened on; switching threads closes it.
  const menuOpen = menuIsOpen && activeThread !== undefined && menuAnchor?.threadId === activeThread.id;
  const goToParentLabel = activeThread?.launchTargetId ? parentLabel(activeThread, threads) : undefined;

  // Keep the active chip in view when the thread changes.
  useEffect(() => {
    chipRefs.current.get(activeKey)?.scrollIntoView?.({ inline: 'nearest', block: 'nearest', behavior: scrollBehavior() });
  }, [activeKey]);

  useEffect(() => {
    if (menuOpen) menuRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
  }, [menuOpen]);

  function focusChip(key: string) {
    const chip = chipRefs.current.get(key);
    if (!chip) return;
    setFocusKey(key);
    chip.focus();
    chip.scrollIntoView?.({ inline: 'nearest', block: 'nearest', behavior: scrollBehavior() });
  }

  function handleTablistKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const index = keys.indexOf(rovingKey);
    let next: number;
    switch (event.key) {
      case 'ArrowRight':
        next = (index + 1) % keys.length;
        break;
      case 'ArrowLeft':
        next = (index - 1 + keys.length) % keys.length;
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = keys.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    const key = keys[next];
    if (key !== undefined) focusChip(key);
  }

  function handleChipClick(thread: TranscriptThreadRef | undefined, event: MouseEvent<HTMLButtonElement>) {
    setFocusKey(null);
    if ((thread?.id ?? MAIN_KEY) !== activeKey) {
      setMenuOpen(false);
      onSelect(thread ? thread.id : null);
      return;
    }
    if (!thread) return;
    if (menuOpen) {
      setMenuOpen(false);
      return;
    }
    const area = menuContainerRef.current?.getBoundingClientRect();
    const chip = event.currentTarget.getBoundingClientRect();
    const left = area ? Math.max(0, Math.min(chip.left - area.left, area.width - MENU_MIN_WIDTH_PX)) : 0;
    setMenuAnchor({ threadId: thread.id, left });
    setMenuOpen(true);
  }

  function closeMenuAndFocusChip() {
    setMenuOpen(false);
    chipRefs.current.get(activeKey)?.focus();
  }

  function handleMenuKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeMenuAndFocusChip();
      return;
    }
    if (event.key === 'Tab') {
      setMenuOpen(false);
      return;
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    const items = Array.from(menuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
    const index = items.findIndex((item) => item === document.activeElement);
    const next = event.key === 'ArrowDown' ? (index + 1) % items.length : (index - 1 + items.length) % items.length;
    items[next]?.focus();
  }

  function goToParent() {
    if (!activeThread?.launchTargetId || activeThread.parentThreadId === undefined) return;
    setMenuOpen(false);
    onSelect(activeThread.parentThreadId, activeThread.launchTargetId);
  }

  function copyLink() {
    if (!activeThread) return;
    const url = `${window.location.origin}/sessions/${sessionId}?tab=transcript&agent=${encodeURIComponent(activeThread.id)}`;
    copy(url).then(
      () => window.setTimeout(() => setMenuOpen(false), COPIED_MS),
      () => setMenuOpen(false),
    );
  }

  function step(direction: 1 | -1) {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    scroller.scrollBy({ left: direction * STEP_RATIO * scroller.clientWidth, behavior: scrollBehavior() });
  }

  function renderChip(thread: TranscriptThreadRef | undefined) {
    const key = thread?.id ?? MAIN_KEY;
    const isActive = key === activeKey;
    const hasMenu = isActive && thread !== undefined;
    const chip = (
      <button
        key={key}
        ref={(el) => {
          if (el) chipRefs.current.set(key, el);
          else chipRefs.current.delete(key);
        }}
        type="button"
        role="tab"
        aria-selected={isActive}
        aria-label={thread ? `${thread.label}, ${THREAD_STATUS_LABEL[threadStatus(thread)]}` : undefined}
        aria-haspopup={hasMenu ? 'menu' : undefined}
        aria-expanded={hasMenu ? menuOpen : undefined}
        tabIndex={key === rovingKey ? 0 : -1}
        className={cx(styles.chip, isActive && styles.chipActive)}
        onFocus={() => setFocusKey(key)}
        onClick={(event) => handleChipClick(thread, event)}
      >
        {thread && isNestedThread(thread) && (
          <span className={styles.nestedMark} data-nested aria-hidden="true">
            {NestedGlyph}
          </span>
        )}
        {thread && <ThreadStatusGlyph status={threadStatus(thread)} />}
        <span className={styles.label}>{thread ? thread.label : 'Main'}</span>
        {hasMenu && (
          <span className={styles.caret} aria-hidden="true">
            {ChevronIcon}
          </span>
        )}
      </button>
    );
    if (!thread) return chip;
    return (
      <Tooltip key={key} content={<ChipDetails thread={thread} threads={threads} />} disabled={menuOpen}>
        {chip}
      </Tooltip>
    );
  }

  return (
    <div className={styles.bar}>
      <div className={styles.threadsArea} ref={menuContainerRef}>
        <div role="tablist" aria-label="Transcript threads" className={styles.tablist} onKeyDown={handleTablistKeyDown}>
          {renderChip(undefined)}
          <span className={styles.divider} aria-hidden="true" />
          <div className={styles.scrollArea}>
            {/* Mouse affordances only: keyboard users move with the arrow keys, which scroll too. */}
            {overflow.start && (
              <button
                type="button"
                tabIndex={-1}
                aria-hidden="true"
                aria-label="Scroll subagents left"
                className={cx(styles.step, styles.stepStart)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => step(-1)}
              >
                {ChevronIcon}
              </button>
            )}
            <div
              ref={scrollerRef}
              data-testid="thread-scroller"
              className={cx(styles.scroller, fadeClassFor(overflow))}
              onScroll={menuIsOpen ? () => setMenuOpen(false) : undefined}
            >
              {threads.map((thread) => renderChip(thread))}
            </div>
            {overflow.end && (
              <button
                type="button"
                tabIndex={-1}
                aria-hidden="true"
                aria-label="Scroll subagents right"
                className={cx(styles.step, styles.stepEnd)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => step(1)}
              >
                {ChevronIcon}
              </button>
            )}
          </div>
        </div>

        {menuOpen && menuAnchor && (
          <div
            ref={menuRef}
            role="menu"
            aria-label="Subagent actions"
            className={cx(dropdownStyles.popover, dropdownStyles.menu)}
            style={{ left: menuAnchor.left }}
            onKeyDown={handleMenuKeyDown}
          >
            {goToParentLabel !== undefined && (
              <button type="button" role="menuitem" tabIndex={-1} className={dropdownStyles.menuItem} onClick={goToParent}>
                Go to parent ({goToParentLabel})
              </button>
            )}
            <button type="button" role="menuitem" tabIndex={-1} className={dropdownStyles.menuItem} onClick={copyLink}>
              {copied ? 'Copied' : 'Copy link to this subagent'}
            </button>
          </div>
        )}
      </div>
      <span className={styles.divider} aria-hidden="true" />
      <AllThreadsDropdown threads={threads} activeThreadId={activeThreadId} onSelect={onSelect} />
    </div>
  );
}
