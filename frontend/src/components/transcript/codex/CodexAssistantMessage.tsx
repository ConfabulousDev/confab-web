// Renders a Codex assistant message. `phase: 'commentary'` styling is lighter
// weight than the default/final styling so commentary is visually subordinate
// to the final answer in the same turn.

import type { CodexAssistantItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
import styles from './CodexMessage.module.css';

export interface CodexAssistantMessageProps {
  item: CodexAssistantItem;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Speaker kind differs from the previous speaker (tool_call doesn't count). */
  isNewSpeaker?: boolean;
}

export default function CodexAssistantMessage({
  item,
  isSelected,
  isNewSpeaker,
}: CodexAssistantMessageProps) {
  const phaseClass = item.phase === 'commentary' ? styles.commentary : styles.final;
  const className = cx(
    styles.message,
    styles.assistant,
    phaseClass,
    isSelected && styles.selected,
    isNewSpeaker && styles.newSpeaker,
  );
  return (
    <div
      className={className}
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
        <CodexMessageBody text={item.text} />
      </div>
    </div>
  );
}
