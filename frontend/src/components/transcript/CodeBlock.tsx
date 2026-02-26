import { useState, useMemo } from 'react';
import Prism from 'prismjs';
import { useCopyToClipboard } from '@/hooks';
import { stripAnsi } from '@/utils';
import { escapeHtml, getHighlightClass, highlightTextInHtml } from '@/utils/highlightSearch';

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

// Import Prism light theme (dark mode overrides in CodeBlock.module.css)
import 'prismjs/themes/prism.css';
import styles from './CodeBlock.module.css';

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

interface CodeBlockProps {
  code: string;
  language?: string;
  showLineNumbers?: boolean;
  maxHeight?: string;
  truncateLines?: number;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
}

function CodeBlock({
  code,
  language = 'plain',
  showLineNumbers = false,
  maxHeight = 'none',
  truncateLines = 0,
  searchQuery,
  isCurrentSearchMatch,
}: CodeBlockProps) {
  const { copy, copied } = useCopyToClipboard();
  const [showingFull, setShowingFull] = useState(false);

  // Strip ANSI codes and check if code needs truncation
  const { displayCode, isTruncated } = useMemo(() => {
    const cleanCode = stripAnsi(code);
    if (truncateLines > 0 && !showingFull) {
      const lines = cleanCode.split('\n');
      if (lines.length > truncateLines) {
        return {
          displayCode: lines.slice(0, truncateLines).join('\n'),
          isTruncated: true,
        };
      }
    }
    return {
      displayCode: cleanCode,
      isTruncated: false,
    };
  }, [code, truncateLines, showingFull]);

  // Auto-expand truncated code blocks when search query matches only in hidden content.
  // Uses the React-recommended "adjust state during render" pattern to avoid cascading effects.
  const [prevSearchQuery, setPrevSearchQuery] = useState(searchQuery);
  if (prevSearchQuery !== searchQuery) {
    setPrevSearchQuery(searchQuery);
    if (searchQuery && searchQuery.trim() && !showingFull && truncateLines > 0) {
      const cleanCode = stripAnsi(code);
      const needle = searchQuery.toLowerCase();
      if (cleanCode.toLowerCase().includes(needle)) {
        const lines = cleanCode.split('\n');
        if (lines.length > truncateLines) {
          const truncatedText = lines.slice(0, truncateLines).join('\n');
          if (!truncatedText.toLowerCase().includes(needle)) {
            setShowingFull(true);
          }
        }
      }
    }
  }

  // Highlight code synchronously via useMemo (not useEffect)
  const highlightedCode = useMemo(() => {
    const lang = normalizeLanguage(language);
    let highlighted: string;

    // Check if language is supported
    if (lang === 'plain' || !Prism.languages[lang]) {
      highlighted = escapeHtml(displayCode);
    } else {
      try {
        highlighted = Prism.highlight(displayCode, Prism.languages[lang], lang);
      } catch (e) {
        console.warn(`Failed to highlight code with language '${lang}':`, e);
        highlighted = escapeHtml(displayCode);
      }
    }

    // Apply search highlighting on top of syntax highlighting
    if (searchQuery) {
      highlighted = highlightTextInHtml(highlighted, searchQuery, getHighlightClass(isCurrentSearchMatch ?? false));
    }

    return highlighted;
  }, [displayCode, language, searchQuery, isCurrentSearchMatch]);

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
