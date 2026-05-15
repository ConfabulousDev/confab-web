// Renders a Codex user prompt. The body uses the shared `CodexMessageBody`
// so user + assistant text flow through the same markdown / JSON pretty-print
// pipeline that Claude's `ContentBlock` uses (CF-358).

import type { CodexUserItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
import CodexRowActions from './CodexRowActions';
import styles from './CodexMessage.module.css';

export interface CodexUserMessageProps {
  item: CodexUserItem;
  /**
   * Session ID for the per-row copy-link URL. Optional so the renderer can
   * be used in isolation (e.g. Storybook, focused unit tests) without the
   * row chrome; the timeline always passes it in production.
   */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Speaker kind differs from the previous speaker (tool_call doesn't count). */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target — adds the accent pulse. */
  isDeepLinkTarget?: boolean;
  /** Skip to next same-kind row (CF-360). */
  onSkipToNext?: () => void;
  /** Skip to previous same-kind row (CF-360). */
  onSkipToPrevious?: () => void;
  /** Human-readable kind for aria-label (CF-360). */
  kindLabel?: string;
}

export default function CodexUserMessage({
  item,
  sessionId,
  isSelected,
  isNewSpeaker,
  isDeepLinkTarget,
  onSkipToNext,
  onSkipToPrevious,
  kindLabel,
}: CodexUserMessageProps) {
  const className = cx(
    styles.message,
    styles.user,
    isSelected && styles.selected,
    isNewSpeaker && styles.newSpeaker,
    isDeepLinkTarget && styles.deepLinkTarget,
  );
  return (
    <div className={className} data-kind="user">
      <div className={styles.header}>
        <span className={styles.role}>User</span>
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
        {sessionId && (
          <CodexRowActions
            sessionId={sessionId}
            lineId={item.lineId}
            copyText={item.text}
            onSkipToNext={onSkipToNext}
            onSkipToPrevious={onSkipToPrevious}
            kindLabel={kindLabel ?? 'user prompt'}
          />
        )}
      </div>
      <div className={styles.body}>
        <CodexMessageBody text={item.text} />
      </div>
    </div>
  );
}
