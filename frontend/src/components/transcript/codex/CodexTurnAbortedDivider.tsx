// CF-368: divider for an aborted turn (`event_msg.turn_aborted`).
//
// Mirrors `CodexCompactedDivider`'s shape — dashed warning border, muted
// text, copy-link-only chrome. The Codex upstream emits this when the user
// presses Esc mid-turn (`reason: 'interrupted'`), when the turn is replaced
// by a follow-up submission, when a review session ends, or when the budget
// cap is hit.
//
// Visible label:
//   Turn aborted [· <reason>] [· <duration>]
// Each segment is dropped when its field is empty / zero, so the smallest
// possible label is plain `Turn aborted` with the timestamp on the right.

import type { CodexTurnAbortedItem } from '@/types/codexRenderItem';
import { renderTextWithHighlight } from '@/utils/renderHighlight';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp, formatDurationMs } from './codexFormat';
import CodexRowActions from './CodexRowActions';
import styles from './CodexDividers.module.css';

export interface CodexTurnAbortedDividerProps {
  item: CodexTurnAbortedItem;
  /** Session ID for the per-row copy-link URL (CF-360). Optional in tests. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for turn_aborted (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** CF-359: search query — wraps matches in `<mark>` inside the label. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
}

/**
 * Build the divider's visible label. Exported so `extractCodexItemText`
 * (CF-359) can put the exact rendered string into the search index — keeps
 * the renderer and the search projection from drifting (mirrors how
 * `compactedLabel` is shared).
 */
// eslint-disable-next-line react-refresh/only-export-components
export function turnAbortedLabel(reason: string, durationMs: number): string {
  const parts = ['Turn aborted'];
  if (reason) parts.push(reason);
  if (durationMs > 0) parts.push(formatDurationMs(durationMs));
  return parts.join(' · ');
}

export default function CodexTurnAbortedDivider({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  searchQuery,
  isCurrentSearchMatch,
}: CodexTurnAbortedDividerProps) {
  const label = turnAbortedLabel(item.reason, item.durationMs);
  const className = cx(
    styles.turnAborted,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );

  return (
    <div className={className} data-kind="turn_aborted">
      <span>{renderTextWithHighlight(label, searchQuery, isCurrentSearchMatch)}</span>
      <span className={styles.turnAbortedTimestamp}>
        {formatCodexTimestamp(item.timestamp)}
      </span>
      {sessionId && (
        <CodexRowActions
          sessionId={sessionId}
          lineId={item.lineId}
          kindLabel="turn-aborted marker"
        />
      )}
    </div>
  );
}
