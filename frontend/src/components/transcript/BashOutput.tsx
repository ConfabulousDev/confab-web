import { useMemo } from 'react';
import { useCopyToClipboard } from '@/hooks';
import { stripAnsi } from '@/utils';
import { escapeHtml, getHighlightClass, highlightTextInHtml } from '@/utils/highlightSearch';
import styles from './BashOutput.module.css';

interface BashOutputProps {
  output: string;
  command?: string;
  exitCode?: number | null;
  maxHeight?: string;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
  // Bash `toolUseResult` metadata (Claude Code >= 2.1.143).
  interrupted?: boolean;
  isImage?: boolean;
  noOutputExpected?: boolean;
  returnCodeInterpretation?: string;
  persistedOutputPath?: string;
  persistedOutputSize?: number;
}

/** Humanize a byte count as B / KB / MB (binary units). */
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  return `${(kb / 1024).toFixed(1)} MB`;
}

function BashOutput({
  output,
  command = '',
  exitCode = null,
  maxHeight = '400px',
  searchQuery,
  isCurrentSearchMatch,
  interrupted = false,
  isImage = false,
  noOutputExpected = false,
  returnCodeInterpretation,
  persistedOutputPath,
  persistedOutputSize,
}: BashOutputProps) {
  const { copy, copied } = useCopyToClipboard();

  // Format the output
  const cleanOutput = useMemo(() => stripAnsi(output), [output]);
  const hasError = exitCode !== null && exitCode !== 0;

  // Build highlighted HTML for the output
  const outputHtml = useMemo(() => {
    let html = escapeHtml(cleanOutput);
    if (searchQuery) {
      html = highlightTextInHtml(html, searchQuery, getHighlightClass(isCurrentSearchMatch ?? false));
    }
    return html;
  }, [cleanOutput, searchQuery, isCurrentSearchMatch]);

  // `isImage: true` means the output is raw image bytes rather than text — don't
  // dump them. We don't decode/render the image this round (no real fixture to
  // verify the encoding); a label suffices. The empty-output hint only applies
  // to genuinely empty text output, not the image case.
  const showNoOutputHint = !isImage && noOutputExpected && cleanOutput.trim() === '';

  function renderBody() {
    if (isImage) {
      return <div className={styles.imageNotice}>Output is an image</div>;
    }
    if (showNoOutputHint) {
      return <div className={styles.noOutputHint}>No output expected (command produced no stdout)</div>;
    }
    return (
      <div className={styles.bashContent} style={{ maxHeight }}>
        <pre dangerouslySetInnerHTML={{ __html: outputHtml }} />
      </div>
    );
  }

  const persistedSuffix =
    typeof persistedOutputSize === 'number' && persistedOutputSize > 0
      ? ` — ${formatBytes(persistedOutputSize)} persisted to `
      : ' — persisted to ';

  return (
    <div className={`${styles.bashOutput} ${hasError ? styles.error : ''}`}>
      <button className={styles.copyBtn} onClick={() => copy(output)} title="Copy output to clipboard">
        {copied ? '✓' : '📋'}
      </button>
      {command && (
        <div className={styles.bashPrompt}>
          <span className={styles.promptSymbol}>$</span>
          <span className={styles.command}>{command}</span>
        </div>
      )}
      {interrupted && (
        <div className={styles.badgeRow}>
          <span className={styles.interruptedBadge} title="The user interrupted this command">interrupted</span>
        </div>
      )}
      {renderBody()}
      {persistedOutputPath && (
        <div className={styles.persistedFooter} title="Full Bash output was written to local disk">
          Output truncated
          {persistedSuffix}
          <span className={styles.persistedPath}>{persistedOutputPath}</span>
        </div>
      )}
      {(hasError || returnCodeInterpretation) && (
        <div className={styles.exitCode}>
          {hasError && (
            <>
              <span className={styles.exitLabel}>Exit code:</span>
              <span className={styles.exitValue}>{exitCode}</span>
            </>
          )}
          {returnCodeInterpretation && (
            <span className={styles.exitInterpretation}>{returnCodeInterpretation}</span>
          )}
        </div>
      )}
    </div>
  );
}

export default BashOutput;
