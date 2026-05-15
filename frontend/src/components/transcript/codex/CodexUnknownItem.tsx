// Forward-compat fallback row for unrecognized Codex line shapes.
// Renders a small chip with the raw JSON behind a click-to-expand so a
// new line type lands somewhere visible instead of being silently dropped.

import type { CodexUnknownItem as CodexUnknownItemType } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatCodexTimestamp, stringifyForDisplay } from './codexFormat';
import CodexRowActions from './CodexRowActions';
import styles from './CodexDividers.module.css';

export interface CodexUnknownItemProps {
  item: CodexUnknownItemType;
  /** Session ID for the per-row copy-link URL (CF-360). Optional in tests. */
  sessionId?: string;
  /** Hover/click selection — adds the .selected ring. */
  isSelected?: boolean;
  /** Never fires for unknown (not a speaker). Accepted for shape uniformity. */
  isNewSpeaker?: boolean;
  /** CF-360: this row is the deep-link landing target. */
  isDeepLinkTarget?: boolean;
}

export default function CodexUnknownItem({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
}: CodexUnknownItemProps) {
  const className = cx(
    styles.unknown,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
  );
  return (
    <details className={className} data-kind="unknown">
      <summary>
        <span>Unrecognized line</span>
        <span className={styles.unknownTimestamp}>
          {formatCodexTimestamp(item.timestamp)}
        </span>
        {sessionId && (
          <CodexRowActions
            sessionId={sessionId}
            lineId={item.lineId}
            copyText={stringifyForDisplay(item.rawLine)}
            kindLabel="unrecognized row"
          />
        )}
      </summary>
      <pre className={styles.unknownRaw}>{stringifyForDisplay(item.rawLine)}</pre>
    </details>
  );
}
