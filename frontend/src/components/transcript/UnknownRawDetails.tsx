// CF-574: provider-agnostic shell for an UNKNOWN transcript row. Renders a
// dashed-border collapsible <details> with the raw line (pre-stringified by the
// caller) behind a click-to-expand, plus summary slots for a right-aligned aside
// (timestamp) and actions (Report button, per-row actions). Shared by the Codex
// and OpenCode unknown rows so the click-to-expand + search-highlight behavior
// and the selection/deep-link/search rings live in exactly one place.

import { useMemo, useState, type ReactNode } from 'react';
import { escapeHtml, getHighlightClass, highlightTextInHtml } from '@/utils/highlightSearch';
import { cx } from '@/utils/utils';
import styles from './UnknownRawDetails.module.css';

interface UnknownRawDetailsProps {
  /** Left-hand summary label, e.g. "Unrecognized line". */
  label: string;
  /** Pre-stringified raw line; the provider controls its own JSON formatting. */
  rawText: string;
  /** Right-aligned summary content (e.g. a formatted timestamp). */
  summaryAside?: ReactNode;
  /** Summary action slot (Report button, per-row actions). Clicks here do not
   *  toggle the <details>. */
  actions?: ReactNode;
  /** Selection ring (hover/click). */
  isSelected?: boolean;
  /** CF-360: deep-link landing target — pulse ring. */
  isDeepLinkTarget?: boolean;
  /** CF-359: search query — highlights matches inside the raw <pre>. */
  searchQuery?: string;
  /** CF-359: this row is the active (n-of-N) match — amber ring + highlight. */
  isCurrentSearchMatch?: boolean;
}

export default function UnknownRawDetails({
  label,
  rawText,
  summaryAside,
  actions,
  isSelected,
  isDeepLinkTarget,
  searchQuery,
  isCurrentSearchMatch,
}: UnknownRawDetailsProps) {
  const queryMatches =
    !!searchQuery && rawText.toLowerCase().includes(searchQuery.toLowerCase());

  // Controlled `open` so the user can still toggle. Auto-open on the rising edge
  // of a search match so the highlighted <mark> is visible without an extra
  // click (mirrors the prior Codex behavior; React "adjust state on prop change"
  // pattern).
  const [open, setOpen] = useState(false);
  const [prevQueryMatches, setPrevQueryMatches] = useState(false);
  if (queryMatches !== prevQueryMatches) {
    setPrevQueryMatches(queryMatches);
    if (queryMatches) setOpen(true);
  }

  const rawHtml = useMemo(() => {
    let html = escapeHtml(rawText);
    if (searchQuery) {
      html = highlightTextInHtml(html, searchQuery, getHighlightClass(isCurrentSearchMatch ?? false));
    }
    return html;
  }, [rawText, searchQuery, isCurrentSearchMatch]);

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
        <span>{label}</span>
        <span className={styles.right}>
          {summaryAside}
          {actions && (
            // Keep clicks on actions (links/buttons) from toggling the <details>.
            <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
              {actions}
            </span>
          )}
        </span>
      </summary>
      <pre className={styles.raw} dangerouslySetInnerHTML={{ __html: rawHtml }} />
    </details>
  );
}
