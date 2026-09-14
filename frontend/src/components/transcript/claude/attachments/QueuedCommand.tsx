import type { QueuedCommandAttachment } from '@/types';
import { renderMarkdownToHtml } from '@/utils';
import SubagentCard from '../SubagentCard';
import { findNotifiedAgent, parseTaskNotification } from '../claudeAgentIndex';
import { useClaudeThread } from '../claudeThreadContext';
import styles from './QueuedCommand.module.css';

interface QueuedCommandProps {
  attachment: QueuedCommandAttachment;
}

/**
 * Renders a queued-command attachment. Branches on `commandMode`:
 *   - `task-notification` for a known subagent → "Subagent finished" card
 *     (et0r D5), raw XML collapsed underneath
 *   - other `task-notification` (e.g. background commands) → raw XML in a
 *     monospace <pre>
 *   - anything else → markdown via the shared renderer
 * (per CF-346 decision #7).
 */
export default function QueuedCommand({ attachment }: QueuedCommandProps) {
  const { agentIndex, onOpenThread } = useClaudeThread();
  const { prompt, commandMode } = attachment;
  const isTaskNotification = commandMode === 'task-notification';
  const notification = isTaskNotification ? parseTaskNotification(prompt) : null;
  const agent = notification ? findNotifiedAgent(agentIndex, notification) : undefined;

  if (notification && agent) {
    return (
      <SubagentCard
        variant="finished"
        agent={agent}
        status={notification.status}
        summary={notification.summary}
        rawLabel="Raw notification"
        onOpenThread={onOpenThread}
      >
        <pre className={styles.xmlBody}>{prompt}</pre>
      </SubagentCard>
    );
  }

  return (
    <div className={styles.queued}>
      <div className={styles.header}>
        <span className={styles.badge}>queued</span>
        {commandMode && <span className={styles.mode}>{commandMode}</span>}
      </div>
      {isTaskNotification ? (
        <pre className={styles.xmlBody}>{prompt}</pre>
      ) : (
        <div
          className={styles.markdownBody}
          dangerouslySetInnerHTML={{ __html: renderMarkdownToHtml(prompt) }}
        />
      )}
    </div>
  );
}
