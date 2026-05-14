// Divider shown where Codex compacted N earlier messages into a summary.

import type { CodexCompactedItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexCompactedDividerProps {
  item: CodexCompactedItem;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for compacted (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
}

export default function CodexCompactedDivider({ item, isSelected }: CodexCompactedDividerProps) {
  const label =
    item.replacementCount > 0
      ? `Context compacted (${item.replacementCount} earlier message${
          item.replacementCount === 1 ? '' : 's'
        })`
      : 'Context compacted';

  const className = cx(styles.compacted, isSelected && styles.selected);
  return (
    <div className={className} data-kind="compacted">
      <span>{label}</span>
      <span className={styles.compactedTimestamp}>
        {formatCodexTimestamp(item.timestamp)}
      </span>
    </div>
  );
}
