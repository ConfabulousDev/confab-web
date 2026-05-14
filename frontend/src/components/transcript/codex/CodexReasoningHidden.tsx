// Minimal "(reasoning hidden)" marker for opaque encrypted reasoning lines.

import type { CodexReasoningHiddenItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexReasoningHiddenProps {
  item: CodexReasoningHiddenItem;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for reasoning_hidden (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
}

export default function CodexReasoningHidden({ item, isSelected }: CodexReasoningHiddenProps) {
  const className = cx(styles.reasoningHidden, isSelected && styles.selected);
  return (
    <div className={className} data-kind="reasoning_hidden">
      <span className={styles.reasoningIcon} aria-hidden="true">
        🔒
      </span>
      <span>reasoning hidden</span>
      <span className={styles.reasoningTimestamp}>
        {formatCodexTimestamp(item.timestamp)}
      </span>
    </div>
  );
}
