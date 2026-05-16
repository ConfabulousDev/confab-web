// Forward-compat fallback row for unrecognized Codex line shapes.
// Renders a small chip with the raw JSON behind a click-to-expand so a
// new line type lands somewhere visible instead of being silently dropped.

import { useMemo, useState } from 'react';
import type { CodexUnknownItem as CodexUnknownItemType } from '@/types/codexRenderItem';
import { escapeHtml, getHighlightClass, highlightTextInHtml } from '@/utils/highlightSearch';
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
  /** CF-359: search query — highlights matches inside the raw-JSON `<pre>`. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) search match — adds the amber ring. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexUnknownItem({
  item,
  sessionId,
  isSelected,
  isDeepLinkTarget,
  searchQuery,
  isCurrentSearchMatch,
}: CodexUnknownItemProps) {
  const raw = useMemo(() => stringifyForDisplay(item.rawLine), [item.rawLine]);
  const queryMatches =
    !!searchQuery && raw.toLowerCase().includes(searchQuery.toLowerCase());

  // Controlled `open` so the user can still toggle. CF-359 auto-opens the
  // <details> when a search match lands inside so the highlighted <mark>
  // is visible without an extra click — parallel to the Claude view where
  // thinking-block content is always shown inline. We force-open on the
  // rising edge of `queryMatches` (React-recommended "adjust state when a
  // prop changes" pattern; mirrors `useTranscriptSearch.ts`).
  const [open, setOpen] = useState(false);
  const [prevQueryMatches, setPrevQueryMatches] = useState(false);
  if (queryMatches !== prevQueryMatches) {
    setPrevQueryMatches(queryMatches);
    if (queryMatches) setOpen(true);
  }

  const rawHtml = useMemo(() => {
    let html = escapeHtml(raw);
    if (searchQuery) {
      html = highlightTextInHtml(html, searchQuery, getHighlightClass(isCurrentSearchMatch ?? false));
    }
    return html;
  }, [raw, searchQuery, isCurrentSearchMatch]);

  const className = cx(
    styles.unknown,
    isSelected && styles.selected,
    isDeepLinkTarget && styles.deepLinkTarget,
    isCurrentSearchMatch && styles.searchMatch,
  );
  return (
    <details
      className={className}
      data-kind="unknown"
      open={open}
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary>
        <span>Unrecognized line</span>
        <span className={styles.unknownTimestamp}>
          {formatCodexTimestamp(item.timestamp)}
        </span>
        {sessionId && (
          <CodexRowActions
            sessionId={sessionId}
            lineId={item.lineId}
            copyText={raw}
            kindLabel="unrecognized row"
          />
        )}
      </summary>
      <pre className={styles.unknownRaw} dangerouslySetInnerHTML={{ __html: rawHtml }} />
    </details>
  );
}
