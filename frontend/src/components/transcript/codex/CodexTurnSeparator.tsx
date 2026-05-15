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
}

export default function CodexTurnSeparator({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
}: CodexTurnSeparatorProps) {
  const className = cx(
    styles.turnSeparator,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
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
