// kpqz: renders a Codex inter-agent message (`response_item.agent_message`) —
// a subagent's reply to its parent, or the parent's task prompt in a child
// session. Headed `author → recipient` so it never reads as a human prompt;
// the body uses the shared `CodexMessageBody` markdown / JSON pipeline.

import type { CodexAgentMessageItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexMessageBody from './CodexMessageBody';
import CodexMessageImages from './CodexMessageImages';
import RowActions from '../RowActions';
import styles from './CodexMessage.module.css';

interface CodexAgentMessageProps {
  item: CodexAgentMessageItem;
  /** Session ID for the per-row copy-link URL; omit to render without row chrome. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** This row is the deep-link landing target — adds the accent pulse. */
  isDeepLinkTarget?: boolean;
  /** Skip to next agent message. */
  onSkipToNext?: () => void;
  /** Skip to previous agent message. */
  onSkipToPrevious?: () => void;
  /** Human-readable kind for aria-label. */
  kindLabel?: string;
  /** Transcript search query — wraps matches in `<mark>` in the body. */
  searchQuery?: string;
  /** This row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexAgentMessage({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  onSkipToNext,
  onSkipToPrevious,
  kindLabel,
  searchQuery,
  isCurrentSearchMatch,
}: CodexAgentMessageProps) {
  const className = cx(
    styles.message,
    styles.agentMessage,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  return (
    <div className={className} data-kind="agent_message">
      <div className={styles.header}>
        <span className={styles.agentRoute}>
          {item.author} → {item.recipient}
        </span>
        <span className={styles.timestamp}>{formatCodexTimestamp(item.timestamp)}</span>
        {sessionId && (
          <RowActions
            sessionId={sessionId}
            deepLinkMsg={item.timestamp}
            copyText={item.text}
            onSkipToNext={onSkipToNext}
            onSkipToPrevious={onSkipToPrevious}
            kindLabel={kindLabel ?? 'agent message'}
          />
        )}
      </div>
      <div className={styles.body}>
        <CodexMessageBody
          text={item.text}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
        {item.images && (
          <CodexMessageImages images={item.images} altPrefix="Agent-message image" />
        )}
      </div>
    </div>
  );
}
