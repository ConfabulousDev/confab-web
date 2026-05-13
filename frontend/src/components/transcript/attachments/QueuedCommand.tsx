import type { QueuedCommandAttachment } from '@/types';
import { renderMarkdownToHtml } from '@/utils';
import styles from './QueuedCommand.module.css';

interface QueuedCommandProps {
  attachment: QueuedCommandAttachment;
}

/**
 * Renders a queued-command attachment. Branches on `commandMode`:
 *   - `task-notification` → raw XML in a monospace <pre>
 *   - anything else → markdown via the shared renderer
 * (per CF-346 decision #7).
 */
export default function QueuedCommand({ attachment }: QueuedCommandProps) {
  const { prompt, commandMode } = attachment;
  const isTaskNotification = commandMode === 'task-notification';

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
