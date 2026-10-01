// Helpers shared between all 4 transcript providers (Claude, Codex, Cursor,
// OpenCode). Kept narrow: only decision/format/scroll logic with identical
// semantics everywhere. Each provider's own `*VirtualItems.ts` builder owns the
// loop that calls the divider helpers (6h7m), and each pane keeps its own index
// translation and selection state around the scroll helpers (cca0) — this file
// stays a function library, not a generic injection builder (see 6h7m's
// Decision 5: forcing Codex's isNewSpeaker tracking or Claude's
// allIndex/filteredIndex tagging through one shared shape wasn't worth it).

import type { Virtualizer } from '@tanstack/react-virtual';

/**
 * Right-offset (px) for `ScrollNavButtons` when the CostBar is visible. Both
 * Claude (`MessageTimeline`) and Codex (`CodexMessageTimeline`) pass this as
 * `rightOffset` so the floating buttons clear the CostBar / TimelineBar rail.
 *
 * CostBar (22px) + gap (8px from `--spacing-sm`) + default right (24px from
 * `--spacing-xl`) + 2px breathing room = 56px.
 */
export const SCROLL_NAV_COST_MODE_RIGHT = 56;

/** Idle-gap divider threshold, shared by every provider's divider check. */
export const TIME_GAP_THRESHOLD_MS = 5 * 60 * 1000;

/** Result of `shouldShowDivider`: whether to inject a divider row, and
 *  whether it marks a calendar-day change (vs. a same-day idle gap). */
interface DividerDecision {
  show: boolean;
  dayChanged: boolean;
}

/**
 * 6h7m: decide whether a divider row belongs between `currentMs` and the
 * last item that HAD a known timestamp before it (`previousKnownMs` —
 * callers skip timestamp-less items, like Claude's `summary` lines, when
 * tracking this rather than always using the immediately-previous array
 * element; see Decision 7). Pure and epoch-ms-based so every provider funnels
 * through one check regardless of wire format: ISO strings (Claude/Codex/
 * Cursor, parsed via `Date.parse`/`new Date(...).getTime()`) or OpenCode's
 * native epoch-ms `timeCreated`.
 *
 * `show` unifies the day-boundary and pre-existing >5min idle-gap dividers
 * into one divider type (Decision 2): true when the local calendar day
 * changed OR the gap exceeds `TIME_GAP_THRESHOLD_MS`.
 */
export function shouldShowDivider(
  currentMs: number,
  previousKnownMs: number | undefined,
): DividerDecision {
  if (previousKnownMs === undefined) return { show: false, dayChanged: false };

  const dayChanged = !isSameLocalDay(currentMs, previousKnownMs);
  const gapMs = currentMs - previousKnownMs;
  return { show: dayChanged || gapMs > TIME_GAP_THRESHOLD_MS, dayChanged };
}

function isSameLocalDay(aMs: number, bMs: number): boolean {
  const a = new Date(aMs);
  const b = new Date(bMs);
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

/**
 * 6h7m: divider label. When `dayChanged`, always the full "Weekday, Month
 * Day" date (Decision 3) — even when the gap was small (e.g. 11:59pm →
 * 12:01am) — since the divider's job is to name the new date, not describe
 * elapsed time. Otherwise falls back to the pre-existing idle-gap
 * today/not-today time text (`formatTimeSeparator`).
 */
export function formatDividerLabel(currentMs: number, dayChanged: boolean): string {
  const date = new Date(currentMs);
  if (dayChanged) {
    return date.toLocaleDateString('en-US', {
      weekday: 'long',
      month: 'long',
      day: 'numeric',
    });
  }
  return formatTimeSeparator(currentMs);
}

/**
 * Format a timestamp (epoch ms) for the idle-gap (non-day-change) divider
 * label. Today → time-of-day; otherwise short date + time. Internal helper for
 * `formatDividerLabel` — providers should call that, not this, directly.
 */
function formatTimeSeparator(ms: number): string {
  const date = new Date(ms);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const messageDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  if (messageDate.getTime() === today.getTime()) {
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  }
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Attach a document-level Cmd/Ctrl+F intercept that calls `onCmdF` and
 * preventDefaults the browser find dialog. Returns the cleanup. Used by
 * both timeline views to open the transcript search bar with the same
 * keybinding the browser would otherwise hijack.
 */
export function addCmdFListener(onCmdF: () => void): () => void {
  function handleKeyDown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
      e.preventDefault();
      onCmdF();
    }
  }
  document.addEventListener('keydown', handleKeyDown);
  return () => document.removeEventListener('keydown', handleKeyDown);
}

/**
 * Repeatedly call `action` across animation frames until `shouldStop` returns
 * true or `maxAttempts` is reached. Used by both timeline views for virtual
 * scroll positioning, where item sizes are estimated until measured and a
 * single `scrollToIndex` call lands short of the target.
 */
export function retryOnAnimationFrame(
  action: () => void,
  shouldStop: () => boolean,
  maxAttempts = 5,
): void {
  function attempt(n: number): void {
    action();
    if (n < maxAttempts && !shouldStop()) {
      requestAnimationFrame(() => attempt(n + 1));
    }
  }
  attempt(0);
}

/** The slice of a TanStack virtualizer the scroll helpers below drive. */
type ScrollVirtualizer = Pick<
  Virtualizer<HTMLDivElement, Element>,
  'scrollToIndex' | 'getVirtualItems'
>;

/** Frames to let `scrollToIndex` retries settle before locating the `<mark>`. */
const SEARCH_SCROLL_SETTLE_FRAMES = 6;
/** Frames to keep looking for the matched row's first `<mark>`. */
const SEARCH_MARK_MAX_ATTEMPTS = 10;

/**
 * cca0: scroll the current search match's row (`virtualIndex`) to center, then
 * bring the row's first `<mark>` into view. Shared by all 4 transcript panes'
 * search effects; each pane translates its match into a virtual index first.
 *
 * The mark scroll waits `SEARCH_SCROLL_SETTLE_FRAMES` before starting:
 * `scrollToIndex` keeps retrying across frames as row measurements settle, and
 * a retry landing after `scrollIntoView` would override it. It then retries
 * across frames in case the virtualizer hasn't mounted (or finished rendering)
 * a tall row yet. This is what surfaces matches in rows that weren't mounted.
 *
 * Returns a cancel function; search effects return it as their cleanup so a
 * superseded match never steals the scroll.
 */
export function scrollToSearchMatch(
  virtualizer: Pick<ScrollVirtualizer, 'scrollToIndex'>,
  getScrollEl: () => HTMLElement | null,
  virtualIndex: number,
): () => void {
  retryOnAnimationFrame(
    () => virtualizer.scrollToIndex(virtualIndex, { align: 'center' }),
    () => false,
  );

  let cancelled = false;
  function scrollToMark(attempt: number): void {
    if (cancelled || attempt >= SEARCH_MARK_MAX_ATTEMPTS) return;
    const scrollEl = getScrollEl();
    if (!scrollEl) return;
    const mark = scrollEl.querySelector(`[data-index="${virtualIndex}"]`)?.querySelector('mark');
    if (mark) {
      mark.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    } else {
      requestAnimationFrame(() => scrollToMark(attempt + 1));
    }
  }
  function delayThenScroll(framesLeft: number): void {
    if (cancelled) return;
    if (framesLeft <= 0) {
      scrollToMark(0);
      return;
    }
    requestAnimationFrame(() => delayThenScroll(framesLeft - 1));
  }
  delayThenScroll(SEARCH_SCROLL_SETTLE_FRAMES);

  return () => {
    cancelled = true;
  };
}

/** cca0: scroll to the first row, retrying until the virtualizer shows index 0. */
export function scrollVirtualizerToStart(virtualizer: ScrollVirtualizer): void {
  retryOnAnimationFrame(
    () => virtualizer.scrollToIndex(0, { align: 'start' }),
    () => virtualizer.getVirtualItems()[0]?.index === 0,
  );
}

/** cca0: scroll to `lastIndex`, retrying until the virtualizer shows it. */
export function scrollVirtualizerToEnd(virtualizer: ScrollVirtualizer, lastIndex: number): void {
  retryOnAnimationFrame(
    () => virtualizer.scrollToIndex(lastIndex, { align: 'end' }),
    () => {
      const visible = virtualizer.getVirtualItems();
      const last = visible[visible.length - 1];
      return !!last && last.index >= lastIndex;
    },
  );
}

/**
 * cca0: the first defined `lookup(items[i])` for `i >= start`, skipping holes
 * and items `lookup` doesn't map; `undefined` when none map. Used by the
 * TimelineBar seek handlers to turn an unfiltered segment start into the first
 * row at or after it that survives the active filter.
 */
export function firstIndexAtOrAfter<T>(
  items: readonly T[],
  start: number,
  lookup: (item: T) => number | undefined,
): number | undefined {
  for (let i = start; i < items.length; i++) {
    const item = items[i];
    if (item === undefined) continue;
    const mapped = lookup(item);
    if (mapped !== undefined) return mapped;
  }
  return undefined;
}
