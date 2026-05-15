// Divider shown where Codex compacted N earlier messages into a summary.

import type { CodexCompactedItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp } from './codexFormat';
import CodexRowActions from './CodexRowActions';
import styles from './CodexDividers.module.css';

export interface CodexCompactedDividerProps {
  item: CodexCompactedItem;
  /** Session ID for the per-row copy-link URL (CF-360). Optional in tests. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for compacted (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
}

export default function CodexCompactedDivider({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
}: CodexCompactedDividerProps) {
  const label =
    item.replacementCount > 0
      ? `Context compacted (${item.replacementCount} earlier message${
          item.replacementCount === 1 ? '' : 's'
        })`
      : 'Context compacted';

  const className = cx(
    styles.compacted,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
  );
  return (
    <div className={className} data-kind="compacted">
      <span>{label}</span>
      <span className={styles.compactedTimestamp}>
        {formatCodexTimestamp(item.timestamp)}
      </span>
      {sessionId && (
        <CodexRowActions
          sessionId={sessionId}
          lineId={item.lineId}
          kindLabel="compaction marker"
        />
      )}
    </div>
  );
}
