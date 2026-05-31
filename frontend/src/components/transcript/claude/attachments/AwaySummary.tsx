import type { SystemMessage } from '@/types';
import { renderMarkdownToHtml } from '@/utils';
import styles from './AwaySummary.module.css';

interface AwaySummaryProps {
  /** A system message with subtype === 'away_summary'. The narrower type is
   *  enforced at the call site (TimelineMessage's dispatch). */
  message: SystemMessage;
}

/**
 * Renders an `away_summary` system row — the "you stepped away, here's the
 * resume context" blurb Claude Code emits when re-entering a session. Reuses
 * the same purple .summary card chrome as canonical summary records
 * (per CF-346 decision #9); the distinguishing 'Resume Summary' role label
 * lives in TimelineMessage's header (via getRoleLabel).
 */
export default function AwaySummary({ message }: AwaySummaryProps) {
  const content = message.content;
  if (!content || content.trim().length === 0) return null;

  return (
    <div
      className={styles.awaySummary}
      dangerouslySetInnerHTML={{ __html: renderMarkdownToHtml(content) }}
    />
  );
}
