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
}

function BashOutput({ output, command = '', exitCode = null, maxHeight = '400px', searchQuery, isCurrentSearchMatch }: BashOutputProps) {
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
      <div className={styles.bashContent} style={{ maxHeight }}>
        <pre dangerouslySetInnerHTML={{ __html: outputHtml }} />
      </div>
      {exitCode !== null && exitCode !== 0 && (
        <div className={styles.exitCode}>
          <span className={styles.exitLabel}>Exit code:</span>
          <span className={styles.exitValue}>{exitCode}</span>
        </div>
      )}
    </div>
  );
}

export default BashOutput;
