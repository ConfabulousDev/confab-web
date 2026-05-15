// Renders a Codex assistant message. `phase: 'commentary'` styling is lighter
// weight than the default/final styling so commentary is visually subordinate
// to the final answer in the same turn.

import type { CodexAssistantItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
import CodexMessageImages from './CodexMessageImages';
import CodexRowActions from './CodexRowActions';
import styles from './CodexMessage.module.css';

export interface CodexAssistantMessageProps {
  item: CodexAssistantItem;
  /**
   * Session ID for the per-row copy-link URL. Optional so the renderer can
   * be used in isolation; timeline always passes it in production.
   */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Speaker kind differs from the previous speaker (tool_call doesn't count). */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
  /** Skip to next same-kind row (CF-360). */
  onSkipToNext?: () => void;
  /** Skip to previous same-kind row (CF-360). */
  onSkipToPrevious?: () => void;
  /** Human-readable kind for aria-label (CF-360). */
  kindLabel?: string;
}

export default function CodexAssistantMessage({
  item,
  sessionId,
  isSelected,
  isNewSpeaker,
  isDeepLinkTarget,
  onSkipToNext,
  onSkipToPrevious,
  kindLabel,
}: CodexAssistantMessageProps) {
  const phaseClass = item.phase === 'commentary' ? styles.commentary : styles.final;
  const className = cx(
    styles.message,
    styles.assistant,
    phaseClass,
    isSelected && styles.selected,
    isNewSpeaker && styles.newSpeaker,
    isDeepLinkTarget && styles.deepLinkTarget,
  );
  const defaultLabel =
    item.phase === 'commentary' ? 'assistant commentary' : 'assistant answer';
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
        {sessionId && (
          <CodexRowActions
            sessionId={sessionId}
            lineId={item.lineId}
            copyText={item.text}
            onSkipToNext={onSkipToNext}
            onSkipToPrevious={onSkipToPrevious}
            kindLabel={kindLabel ?? defaultLabel}
          />
        )}
      </div>
      <div className={styles.body}>
        <CodexMessageBody text={item.text} />
        {item.images && (
          <CodexMessageImages images={item.images} altPrefix="Assistant-generated image" />
        )}
      </div>
    </div>
  );
}
