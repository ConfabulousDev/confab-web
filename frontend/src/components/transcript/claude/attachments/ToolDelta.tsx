import type { DeferredToolsDeltaAttachment, McpInstructionsDeltaAttachment } from '@/types';
import styles from './ToolDelta.module.css';

type ToolDeltaPayload = DeferredToolsDeltaAttachment | McpInstructionsDeltaAttachment;

interface ToolDeltaProps {
  attachment: ToolDeltaPayload;
}

/**
 * Renders a tool-availability delta. Shared between `deferred_tools_delta` and
 * `mcp_instructions_delta` (per CF-346 plan §3) — the only thing that differs
 * is the header text. Added and removed name lists are always expanded as chip
 * lists (per CF-346 decision #12).
 */
export default function ToolDelta({ attachment }: ToolDeltaProps) {
  const isMcp = attachment.type === 'mcp_instructions_delta';
  const added = attachment.addedNames ?? [];
  const removed = attachment.removedNames ?? [];

  return (
    <div className={styles.delta}>
      <div className={styles.header}>
        <span className={styles.badge}>{isMcp ? 'mcp instructions' : 'deferred tools'}</span>
        <span className={styles.summary}>
          +{added.length} / −{removed.length}
        </span>
      </div>
      {added.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionLabel}>Added</div>
          <div className={styles.chipList}>
            {added.map((name) => (
              <span key={`add-${name}`} className={`${styles.chip} ${styles.chipAdded}`}>{name}</span>
            ))}
          </div>
        </div>
      )}
      {removed.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionLabel}>Removed</div>
          <div className={styles.chipList}>
            {removed.map((name) => (
              <span key={`rm-${name}`} className={`${styles.chip} ${styles.chipRemoved}`}>{name}</span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
