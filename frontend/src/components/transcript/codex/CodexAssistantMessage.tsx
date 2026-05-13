// Renders a Codex assistant message. `phase: 'commentary'` styling is lighter
// weight than the default/final styling so commentary is visually subordinate
// to the final answer in the same turn.

import type { CodexAssistantItem } from '@/types/codexRenderItem';
import { formatCodexTimestamp } from './codexFormat';
import styles from './CodexMessage.module.css';

export interface CodexAssistantMessageProps {
  item: CodexAssistantItem;
}

export default function CodexAssistantMessage({ item }: CodexAssistantMessageProps) {
  const phaseClass = item.phase === 'commentary' ? styles.commentary : styles.final;
  return (
    <div
      className={`${styles.message} ${styles.assistant} ${phaseClass}`}
      data-kind="assistant"
      data-phase={item.phase}
    >
      <div className={styles.header}>
        <span className={styles.role}>
          {item.phase === 'commentary' ? 'Assistant (commentary)' : 'Assistant'}
        </span>
        <span className={styles.modelBadge}>{item.model}</span>
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
      </div>
      <div className={styles.body}>
        <pre className={styles.text}>{item.text}</pre>
      </div>
    </div>
  );
}
