// Forward-compat fallback row for unrecognized Codex line shapes.
// Renders a small chip with the raw JSON behind a click-to-expand so a
// new line type lands somewhere visible instead of being silently dropped.

import type { CodexUnknownItem as CodexUnknownItemType } from '@/types/codexRenderItem';
import { formatCodexTimestamp, stringifyForDisplay } from './codexFormat';
import styles from './CodexDividers.module.css';

export interface CodexUnknownItemProps {
  item: CodexUnknownItemType;
}

export default function CodexUnknownItem({ item }: CodexUnknownItemProps) {
  return (
    <details className={styles.unknown} data-kind="unknown">
      <summary>
        <span>Unrecognized line</span>
        <span className={styles.unknownTimestamp}>
          {formatCodexTimestamp(item.timestamp)}
        </span>
      </summary>
      <pre className={styles.unknownRaw}>{stringifyForDisplay(item.rawLine)}</pre>
    </details>
  );
}
