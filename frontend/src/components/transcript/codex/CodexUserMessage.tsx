// Renders a Codex user prompt. The body uses the shared `CodexMessageBody`
// so user + assistant text flow through the same markdown / JSON pretty-print
// pipeline that Claude's `ContentBlock` uses (CF-358).

import type { CodexUserItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
import styles from './CodexMessage.module.css';

export interface CodexUserMessageProps {
  item: CodexUserItem;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Speaker kind differs from the previous speaker (tool_call doesn't count). */
  isNewSpeaker?: boolean;
}

export default function CodexUserMessage({ item, isSelected, isNewSpeaker }: CodexUserMessageProps) {
  const className = cx(
    styles.message,
    styles.user,
    isSelected && styles.selected,
    isNewSpeaker && styles.newSpeaker,
  );
  return (
    <div className={className} data-kind="user">
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
