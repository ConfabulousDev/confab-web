// Visible "Turn N — Ms · TTFT Yms" divider between agent turns.

import type { CodexTurnSeparatorItem } from '@/types/codexRenderItem';
import { formatDurationMs } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexTurnSeparatorProps {
  item: CodexTurnSeparatorItem;
}

export default function CodexTurnSeparator({ item }: CodexTurnSeparatorProps) {
  return (
    <div className={styles.turnSeparator} data-kind="turn_separator">
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
