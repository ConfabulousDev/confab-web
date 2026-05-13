// Minimal "(reasoning hidden)" marker for opaque encrypted reasoning lines.

import type { CodexReasoningHiddenItem } from '@/types/codexRenderItem';
import { formatCodexTimestamp } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexReasoningHiddenProps {
  item: CodexReasoningHiddenItem;
}

export default function CodexReasoningHidden({ item }: CodexReasoningHiddenProps) {
  return (
    <div className={styles.reasoningHidden} data-kind="reasoning_hidden">
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
