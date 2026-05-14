// Renders a Codex user prompt. The body uses the shared `CodexMessageBody`
// so user + assistant text flow through the same markdown / JSON pretty-print
// pipeline that Claude's `ContentBlock` uses (CF-358).

import type { CodexUserItem } from '@/types/codexRenderItem';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
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
        <CodexMessageBody text={item.text} />
      </div>
    </div>
  );
}
