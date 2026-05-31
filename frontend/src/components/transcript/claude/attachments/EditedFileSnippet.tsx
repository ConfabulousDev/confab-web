import type { EditedTextFileAttachment } from '@/types';
import CodeBlock from '../../CodeBlock';
import styles from './EditedFileSnippet.module.css';

interface EditedFileSnippetProps {
  attachment: EditedTextFileAttachment;
}

// Map common file extensions to Prism language names (matches CodeBlock's
// languageMap aliases). Anything missing falls back to 'plain'.
const EXTENSION_TO_LANGUAGE: Record<string, string> = {
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  py: 'python',
  go: 'go',
  rs: 'rust',
  json: 'json',
  yml: 'yaml',
  yaml: 'yaml',
  md: 'markdown',
  markdown: 'markdown',
  sh: 'bash',
  bash: 'bash',
  zsh: 'bash',
  sql: 'sql',
  css: 'css',
  html: 'markup',
  xml: 'markup',
};

function languageFromFilename(filename: string): string {
  const dot = filename.lastIndexOf('.');
  if (dot === -1 || dot === filename.length - 1) return 'plain';
  const ext = filename.slice(dot + 1).toLowerCase();
  return EXTENSION_TO_LANGUAGE[ext] ?? 'plain';
}

/**
 * Strip `cat -n` style line-number prefixes (whitespace-padded number + tab)
 * from each line of a snippet. The Claude Code transcript bakes these in;
 * stripping them lets us render the snippet through CodeBlock's normal pipeline
 * (per CF-346 decision #6). If a line doesn't match the prefix pattern we
 * leave it untouched so un-prefixed snippets pass through verbatim.
 */
function stripLineNumberPrefixes(snippet: string): string {
  return snippet
    .split('\n')
    .map((line) => {
      const match = line.match(/^\s*\d+\t(.*)$/);
      return match ? match[1] : line;
    })
    .join('\n');
}

export default function EditedFileSnippet({ attachment }: EditedFileSnippetProps) {
  const { filename, snippet } = attachment;
  const language = languageFromFilename(filename);
  const stripped = stripLineNumberPrefixes(snippet);

  return (
    <div className={styles.editedFile}>
      <div className={styles.header}>
        <span className={styles.badge}>edited file</span>
        <span className={styles.filename}>{filename}</span>
      </div>
      <CodeBlock code={stripped} language={language} maxHeight="500px" />
    </div>
  );
}
