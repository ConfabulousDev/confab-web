// Renders a Codex user prompt. Plain text in a chat-style row.

import type { CodexUserItem } from '@/types/codexRenderItem';
import { formatCodexTimestamp } from './codexFormat';
import styles from './CodexMessage.module.css';

export interface CodexUserMessageProps {
  item: CodexUserItem;
}

export default function CodexUserMessage({ item }: CodexUserMessageProps) {
  return (
    <div className={`${styles.message} ${styles.user}`} data-kind="user">
      <div className={styles.header}>
        <span className={styles.role}>User</span>
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
      </div>
      <div className={styles.body}>
        <pre className={styles.text}>{item.text}</pre>
      </div>
    </div>
  );
}
