import type { SystemMessage } from '@/types';
import { renderMarkdownToHtml } from '@/utils';
import styles from './InformationalBanner.module.css';

interface InformationalBannerProps {
  /** A system message with subtype === 'informational'. The narrower type is
   *  enforced at the call site (ClaudeTimelineMessage's dispatch). */
  message: SystemMessage;
}

// Known severity hints on the informational row's `level` field. Anything else
// (or a missing level) falls back to the neutral 'info' chrome.
const CALLOUT_LEVELS = new Set(['info', 'warning', 'error']);

/**
 * Renders a `system` row with `subtype === 'informational'` — Claude Code's
 * once-per-entry onboarding banner (CC >= 2.1.143), most visibly the "Auto mode
 * lets Claude handle permission prompts automatically …" notice. The `level`
 * field ('info' | 'warning' | 'error') keys the callout chrome; a missing or
 * unrecognized level degrades gracefully to neutral 'info' styling.
 */
export default function InformationalBanner({ message }: InformationalBannerProps) {
  const content = message.content;
  if (!content || content.trim().length === 0) return null;

  const level = message.level && CALLOUT_LEVELS.has(message.level) ? message.level : 'info';

  return (
    <div className={`${styles.callout} ${styles[level]}`} role="note">
      <div
        className={styles.body}
        dangerouslySetInnerHTML={{ __html: renderMarkdownToHtml(content) }}
      />
    </div>
  );
}
