// Visible "Turn N — Ms · TTFT Yms" divider between agent turns.

import type { CodexTurnSeparatorItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatDurationMs } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexTurnSeparatorProps {
  item: CodexTurnSeparatorItem;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for turn separators (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
}

export default function CodexTurnSeparator({ item, isSelected }: CodexTurnSeparatorProps) {
  const className = cx(styles.turnSeparator, isSelected && styles.selected);
  return (
    <div className={className} data-kind="turn_separator">
      <span className={styles.turnLabel}>Turn {item.turnIndex}</span>
      <span className={styles.turnMeta}>
        {formatDurationMs(item.durationMs)}
        {item.timeToFirstTokenMs !== undefined
          ? ` · TTFT ${formatDurationMs(item.timeToFirstTokenMs)}`
          : null}
      </span>
    </div>
  );
}
