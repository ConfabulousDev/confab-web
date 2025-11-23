import { useState, useMemo } from 'react';
import styles from './BashOutput.module.css';

interface BashOutputProps {
  output: string;
  command?: string;
  exitCode?: number | null;
  maxHeight?: string;
}

function BashOutput({ output, command = '', exitCode = null, maxHeight = '400px' }: BashOutputProps) {
  const [copySuccess, setCopySuccess] = useState(false);

  // Parse ANSI color codes (basic support)
  function parseANSI(text: string): string {
    // Remove ANSI escape sequences for now
    // In the future, we could convert them to HTML colors
    return text.replace(/\x1b\[[0-9;]*m/g, '');
  }

  // Format the output
  const cleanOutput = useMemo(() => parseANSI(output), [output]);
  const hasError = exitCode !== null && exitCode !== 0;

  async function copyToClipboard() {
    try {
      await navigator.clipboard.writeText(output);
      setCopySuccess(true);
      setTimeout(() => {
        setCopySuccess(false);
      }, 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }

  return (
    <div className={`${styles.bashOutput} ${hasError ? styles.error : ''}`}>
      <button className={styles.copyBtn} onClick={copyToClipboard} title="Copy output to clipboard">
        {copySuccess ? '✓' : '📋'}
      </button>
      {command && (
        <div className={styles.bashPrompt}>
          <span className={styles.promptSymbol}>$</span>
          <span className={styles.command}>{command}</span>
        </div>
      )}
      <div className={styles.bashContent} style={{ maxHeight }}>
        <pre>{cleanOutput}</pre>
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
