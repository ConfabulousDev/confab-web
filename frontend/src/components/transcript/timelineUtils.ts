// Helpers shared between all 4 transcript providers (Claude, Codex, Cursor,
// OpenCode). Kept narrow: only decision/format/scroll-retry logic with
// identical semantics everywhere. Each provider's own `*VirtualItems.ts`
// builder owns the loop that calls these (6h7m) — this file stays a pure
// function library, not a generic injection builder (see that ticket's
// Decision 5: forcing Codex's isNewSpeaker tracking or Claude's
// allIndex/filteredIndex tagging through one shared shape wasn't worth it).

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
export interface DividerDecision {
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
