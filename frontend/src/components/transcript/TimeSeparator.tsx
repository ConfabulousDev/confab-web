// 6h7m: shared presentational divider row (line — label — line) used by all 4
// transcript providers (Claude, Codex, Cursor, OpenCode) for both the
// pre-existing >5min idle-gap divider and the new day-boundary divider (the
// two are merged into one divider type — see timelineUtils.ts's
// `shouldShowDivider`/`formatDividerLabel`). Lives alongside TimelineBar/
// CostBar, this codebase's existing precedent for shared, provider-agnostic
// transcript UI. Pure presentational: callers own the day-vs-gap decision and
// the label text; this component only renders it.

import styles from './TimeSeparator.module.css';

export interface TimeSeparatorProps {
  /** Divider text — either the >5min idle-gap time text or the full
   *  "Weekday, Month Day" day-boundary date (see `formatDividerLabel`). */
  label: string;
  /**
   * True for Cursor's synthesized/interpolated per-row timestamps (ce79) —
   * renders the same muted `~` prefix + "estimated" tooltip convention used
   * elsewhere on Cursor rows, so the divider doesn't invent a 5th bespoke
   * treatment for the same caveat (Decision 8).
   */
  estimated?: boolean;
}

/** Tooltip shown on every estimated Cursor time (row headers and dividers) —
 *  Cursor transcripts have no per-message timestamps, so these are
 *  interpolated, not real (ce79). Shared with `CursorTranscriptPane`. */
export const ESTIMATED_TIME_TOOLTIP =
  'Estimated — Cursor transcripts have no per-message timestamps.';

export function TimeSeparator({ label, estimated }: TimeSeparatorProps) {
  return (
    <div className={styles.timeSeparator}>
      <span className={styles.separatorLine} />
      <span className={styles.separatorText} title={estimated ? ESTIMATED_TIME_TOOLTIP : undefined}>
        {estimated ? <span className={styles.estimatedTilde}>~</span> : null}
        {label}
      </span>
      <span className={styles.separatorLine} />
    </div>
  );
}

export default TimeSeparator;
