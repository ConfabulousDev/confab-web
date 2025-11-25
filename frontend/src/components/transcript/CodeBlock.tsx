import { useState, useEffect, useMemo } from 'react';
import Prism from 'prismjs';
import { useCopyToClipboard } from '@/hooks';

// Import core languages
import 'prismjs/components/prism-bash';
import 'prismjs/components/prism-typescript';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-json';
import 'prismjs/components/prism-python';
import 'prismjs/components/prism-go';
import 'prismjs/components/prism-markdown';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-sql';
import 'prismjs/components/prism-css';
import 'prismjs/components/prism-markup'; // HTML/XML

// Import a clean theme
import 'prismjs/themes/prism.css';
import styles from './CodeBlock.module.css';

interface CodeBlockProps {
  code: string;
  language?: string;
  showLineNumbers?: boolean;
  maxHeight?: string;
  truncateLines?: number;
}

function CodeBlock({
  code,
  language = 'plain',
  showLineNumbers = false,
  maxHeight = 'none',
  truncateLines = 0,
}: CodeBlockProps) {
  const { copy, copied } = useCopyToClipboard();
  const [showingFull, setShowingFull] = useState(false);
  const [highlightedCode, setHighlightedCode] = useState('');

  // Map common aliases to Prism language names
  const languageMap: Record<string, string> = {
    js: 'javascript',
    ts: 'typescript',
    py: 'python',
    sh: 'bash',
    shell: 'bash',
    yml: 'yaml',
    html: 'markup',
    xml: 'markup',
    txt: 'plain',
    text: 'plain',
  };

  function normalizeLanguage(lang: string): string {
    const normalized = lang.toLowerCase().trim();
    return languageMap[normalized] || normalized;
  }

  function escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // Check if code needs truncation
  const { displayCode, isTruncated } = useMemo(() => {
    if (truncateLines > 0 && !showingFull) {
      const lines = code.split('\n');
      if (lines.length > truncateLines) {
        return {
          displayCode: lines.slice(0, truncateLines).join('\n'),
          isTruncated: true,
        };
      }
    }
    return {
      displayCode: code,
      isTruncated: false,
    };
  }, [code, truncateLines, showingFull]);

  // Highlight code when displayCode or language changes
  useEffect(() => {
    const lang = normalizeLanguage(language);

    // Check if language is supported
    if (lang === 'plain' || !Prism.languages[lang]) {
      setHighlightedCode(escapeHtml(displayCode));
      return;
    }

    try {
      const highlighted = Prism.highlight(displayCode, Prism.languages[lang], lang);
      setHighlightedCode(highlighted);
    } catch (e) {
      console.warn(`Failed to highlight code with language '${lang}':`, e);
      setHighlightedCode(escapeHtml(displayCode));
    }
  }, [displayCode, language]);

  function toggleFullView() {
    setShowingFull(!showingFull);
  }

  return (
    <div className={`${styles.codeBlock} ${showLineNumbers ? styles.lineNumbers : ''}`}>
      <button className={styles.copyBtn} onClick={() => copy(code)} title="Copy to clipboard">
        {copied ? '✓ Copied' : '📋 Copy'}
      </button>
      <pre style={{ maxHeight }}>
        <code className={`language-${normalizeLanguage(language)}`} dangerouslySetInnerHTML={{ __html: highlightedCode }} />
      </pre>
      {isTruncated && (
        <div className={styles.truncateNotice}>
          <span className={styles.truncateText}>{showingFull ? '' : `Showing first ${truncateLines} lines...`}</span>
          <button className={styles.expandToggle} onClick={toggleFullView}>
            {showingFull ? 'Show less' : 'Show all'}
          </button>
        </div>
      )}
    </div>
  );
}

export default CodeBlock;
