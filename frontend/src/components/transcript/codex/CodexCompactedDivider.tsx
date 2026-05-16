// Divider shown where Codex compacted N earlier messages into a summary.

import type { CodexCompactedItem } from '@/types/codexRenderItem';
import { renderTextWithHighlight } from '@/utils/renderHighlight';
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
  /** CF-359: search query — wraps matches in `<mark>` inside the label. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
}

/**
 * Build the divider's visible label. Exported so `extractCodexItemText`
 * (CF-359) can put the exact rendered string into the search index —
 * keeps the renderer and the search projection from drifting.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function compactedLabel(replacementCount: number): string {
  if (replacementCount <= 0) return 'Context compacted';
  const noun = replacementCount === 1 ? 'message' : 'messages';
  return `Context compacted (${replacementCount} earlier ${noun})`;
}

export default function CodexCompactedDivider({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  searchQuery,
  isCurrentSearchMatch,
}: CodexCompactedDividerProps) {
  const label = compactedLabel(item.replacementCount);

  const className = cx(
    styles.compacted,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  return (
    <div className={className} data-kind="compacted">
      <span>{renderTextWithHighlight(label, searchQuery, isCurrentSearchMatch)}</span>
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
