// Divider shown where Codex compacted N earlier messages into a summary.

import type { CodexCompactedItem } from '@/types/codexRenderItem';
import { formatCodexTimestamp } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexCompactedDividerProps {
  item: CodexCompactedItem;
}

export default function CodexCompactedDivider({ item }: CodexCompactedDividerProps) {
  const label =
    item.replacementCount > 0
      ? `Context compacted (${item.replacementCount} earlier message${
          item.replacementCount === 1 ? '' : 's'
        })`
      : 'Context compacted';

  return (
    <div className={styles.compacted} data-kind="compacted">
      <span>{label}</span>
      <span className={styles.compactedTimestamp}>
        {formatCodexTimestamp(item.timestamp)}
      </span>
    </div>
  );
}
