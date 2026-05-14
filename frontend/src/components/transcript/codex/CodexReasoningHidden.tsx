// Minimal "(reasoning hidden)" marker for opaque encrypted reasoning lines.
//
// Forward-compat (CF-358): if a future Codex `reasoning` line ever carries
// plaintext, extend `CodexReasoningHiddenItem` with a `{ decoded: true;
// text: string }` discriminator and render a 💭 collapsible block here,
// mirroring `ContentBlock.tsx` thinking-block treatment. Today's normalizer
// only emits the encrypted/hidden case, so this component stays minimal.

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
