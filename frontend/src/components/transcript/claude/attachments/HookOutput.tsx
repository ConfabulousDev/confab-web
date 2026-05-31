import type { HookSuccessAttachment, HookBlockingErrorAttachment } from '@/types';
import styles from './HookOutput.module.css';

interface HookSuccessProps {
  attachment: HookSuccessAttachment;
}

interface HookBlockingErrorProps {
  attachment: HookBlockingErrorAttachment;
}

/**
 * Format a duration in milliseconds for the header (e.g., "31ms", "1.2s").
 */
function formatDuration(ms: number | undefined): string | null {
  if (ms == null) return null;
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

/**
 * Renders a successful hook execution. Stdout and stderr are always expanded
 * inline (per CF-346 decision #3); empty streams are omitted.
 */
export function HookSuccessOutput({ attachment }: HookSuccessProps) {
  const { hookName, hookEvent, command, stdout, stderr, exitCode, durationMs } = attachment;
  const duration = formatDuration(durationMs);

  return (
    <div className={styles.hook}>
      <div className={styles.header}>
        <span className={styles.badge}>hook</span>
        {hookName && <span className={styles.name}>{hookName}</span>}
        {hookEvent && <span className={styles.event}>{hookEvent}</span>}
        {exitCode != null && <span className={styles.exitCode}>exit {exitCode}</span>}
        {duration && <span className={styles.duration}>{duration}</span>}
      </div>
      {command && <div className={styles.command}>$ {command}</div>}
      {stdout && stdout.trim().length > 0 && (
        <div className={styles.stream}>
          <div className={styles.streamLabel}>stdout</div>
          <pre className={styles.streamBody}>{stdout}</pre>
        </div>
      )}
      {stderr && stderr.trim().length > 0 && (
        <div className={styles.stream}>
          <div className={styles.streamLabel}>stderr</div>
          <pre className={styles.streamBody}>{stderr}</pre>
        </div>
      )}
    </div>
  );
}

/**
 * Renders a hook-blocking-error attachment. Standard card chrome on the outside;
 * the blockingError text shown back to Claude lives in a red panel inside
 * (per CF-346 decision #17).
 */
export function HookBlockingError({ attachment }: HookBlockingErrorProps) {
  const { hookName, hookEvent, blockingError } = attachment;
  return (
    <div className={styles.hook}>
      <div className={styles.header}>
        <span className={`${styles.badge} ${styles.badgeBlocked}`}>hook blocked</span>
        {hookName && <span className={styles.name}>{hookName}</span>}
        {hookEvent && <span className={styles.event}>{hookEvent}</span>}
      </div>
      {blockingError.command && <div className={styles.command}>$ {blockingError.command}</div>}
      <div className={styles.blockingErrorPanel}>
        <div className={styles.blockingErrorLabel}>Blocking error sent to model</div>
        <pre className={styles.blockingErrorBody}>{blockingError.blockingError}</pre>
      </div>
    </div>
  );
}
