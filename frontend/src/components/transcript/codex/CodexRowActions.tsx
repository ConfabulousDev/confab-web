// CF-360: per-row action buttons shared by every Codex renderer.
//
// Renders, into a row's header-right slot:
//   [prev-skip?] [next-skip?] [copy-text?] [copy-link]
//
// - copy-link is always rendered; it builds a deep-link URL of the form
//     ${origin}/sessions/${sessionId}?tab=transcript&msg=${lineId}
//   that matches Claude's TimelineMessage exactly (no trailing `/`).
// - copy-text is shown only when `copyText` is non-empty / non-whitespace.
// - skip buttons appear only when the corresponding callback is provided
//   (parent hides at the ends of a same-kind chain).

import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import styles from './CodexRowActions.module.css';

export interface CodexRowActionsProps {
  sessionId: string;
  lineId: string;
  /** Omitted = no copy-text button. Treated as empty if whitespace-only. */
  copyText?: string;
  /** Both omitted = no skip buttons. Each missing = that direction hidden. */
  onSkipToNext?: () => void;
  onSkipToPrevious?: () => void;
  /** Human-readable kind for aria-label/title, e.g. "exec command". */
  kindLabel?: string;
}

export default function CodexRowActions({
  sessionId,
  lineId,
  copyText,
  onSkipToNext,
  onSkipToPrevious,
  kindLabel,
}: CodexRowActionsProps) {
  const { copy: copyTextHandler, copied: textCopied } = useCopyToClipboard();
  const { copy: copyLinkHandler, copied: linkCopied } = useCopyToClipboard();

  // copyText is shown iff it has at least one non-whitespace character. The
  // button only renders when `showCopyText` is true, so `copyText` is known
  // to be a non-empty string by the time `handleCopyText` runs.
  const showCopyText = (copyText?.trim().length ?? 0) > 0;

  function handleCopyText() {
    void copyTextHandler(copyText ?? '');
  }

  function handleCopyLink() {
    void copyLinkHandler(
      `${window.location.origin}/sessions/${sessionId}?tab=transcript&msg=${lineId}`,
    );
  }

  const label = kindLabel ?? 'row';

  return (
    <span className={styles.actions}>
      {onSkipToPrevious && (
        <button
          type="button"
          className={styles.iconBtn}
          onClick={onSkipToPrevious}
          title={`Previous ${label}`}
          aria-label={`Previous ${label}`}
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="4 10 8 6 12 10" />
          </svg>
        </button>
      )}
      {onSkipToNext && (
        <button
          type="button"
          className={styles.iconBtn}
          onClick={onSkipToNext}
          title={`Next ${label}`}
          aria-label={`Next ${label}`}
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="4 6 8 10 12 6" />
          </svg>
        </button>
      )}
      {showCopyText && (
        <button
          type="button"
          className={`${styles.iconBtn} ${textCopied ? styles.copied : ''}`}
          onClick={handleCopyText}
          title="Copy text"
          aria-label="Copy text"
        >
          {textCopied ? (
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="3.5 8.5 6.5 11.5 12.5 4.5" />
            </svg>
          ) : (
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
              <path d="M10.5 5.5V3.5a1.5 1.5 0 0 0-1.5-1.5H3.5A1.5 1.5 0 0 0 2 3.5V9a1.5 1.5 0 0 0 1.5 1.5h2" />
            </svg>
          )}
        </button>
      )}
      <button
        type="button"
        className={`${styles.iconBtn} ${linkCopied ? styles.copied : ''}`}
        onClick={handleCopyLink}
        title="Copy link to row"
        aria-label="Copy link to row"
      >
        {linkCopied ? (
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="3.5 8.5 6.5 11.5 12.5 4.5" />
          </svg>
        ) : (
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M6.5 9.5a3 3 0 0 0 4.24 0l2-2a3 3 0 0 0-4.24-4.24l-1 1" />
            <path d="M9.5 6.5a3 3 0 0 0-4.24 0l-2 2a3 3 0 0 0 4.24 4.24l1-1" />
          </svg>
        )}
      </button>
    </span>
  );
}
