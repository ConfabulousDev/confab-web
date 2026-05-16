// Visible "Turn N — Ms · TTFT Yms" divider between agent turns.

import type { CodexTurnSeparatorItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatDurationMs } from './codexFormat';
import CodexRowActions from './CodexRowActions';
import styles from './CodexDividers.module.css';

export interface CodexTurnSeparatorProps {
  item: CodexTurnSeparatorItem;
  /** Session ID for the per-row copy-link URL (CF-360). Optional in tests. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for turn separators (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** CF-359: accepted for shape uniformity. Extractor returns "" for this
   *  kind so the row never matches; no highlighting branch needed. */
  searchQuery?: string;
  /** CF-359: accepted for shape uniformity. See `searchQuery` note above. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexTurnSeparator({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  isCurrentSearchMatch,
}: CodexTurnSeparatorProps) {
  const className = cx(
    styles.turnSeparator,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  return (
    <div className={className} data-kind="turn_separator">
      <span className={styles.turnLabel}>Turn {item.turnIndex}</span>
      <span className={styles.turnMeta}>
        {formatDurationMs(item.durationMs)}
        {item.timeToFirstTokenMs !== undefined
          ? ` · TTFT ${formatDurationMs(item.timeToFirstTokenMs)}`
          : null}
      </span>
      {sessionId && (
        <CodexRowActions
          sessionId={sessionId}
          lineId={item.lineId}
          kindLabel="turn separator"
        />
      )}
    </div>
  );
}
