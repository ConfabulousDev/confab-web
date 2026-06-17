// Shared rendering path for Cursor user-prompt + assistant narrative rows (pt81).
//
// Cursor's user/assistant text arrives cleaned upstream — fa3h strips native
// `[REDACTED]`, nfbe extracts the `<user_query>` prompt — so by the time it
// reaches a row it is ready to render as markdown. This routes that cleaned
// text through the shared GFM markdown pipeline (bold / headers / pipe tables /
// links) and preserves Cmd-F by wrapping matches in `<mark>` inside the
// rendered HTML (the same contract Codex uses), so scroll-to-`<mark>` keeps
// working.
//
// Scope (pt81 D2): narrative rows ONLY. Tool rows and injected-context section
// bodies stay monospace `<pre>` and never reach this component.
//
// This is a thin provider-named seam: it delegates to the shared markdown +
// highlight utils rather than re-implementing them (DRY). It mirrors
// `CodexMessageBody`'s JSON-or-markdown fallback so JSON-shaped narrative text
// pretty-prints as a syntax-highlighted code block.

import { renderMarkdownToHtml, tryParseAsJson } from '@/utils';
import { getHighlightClass, highlightTextInHtml } from '@/utils/highlightSearch';
import CodeBlock from '@/components/transcript/CodeBlock';
import styles from './CursorMessageBody.module.css';

export interface CursorMessageBodyProps {
  text: string;
  /** Transcript search query — when set, matches inside the rendered markdown
   *  HTML are wrapped in `<mark>` so Cmd-F + scroll-to-mark keeps working. */
  searchQuery?: string;
  /** Marks this row as the active (n-of-N) match so the highlight uses the
   *  active-match CSS class. */
  isCurrentSearchMatch?: boolean;
}

export default function CursorMessageBody({
  text,
  searchQuery,
  isCurrentSearchMatch,
}: CursorMessageBodyProps) {
  const jsonPretty = tryParseAsJson(text);
  if (jsonPretty) {
    return (
      <CodeBlock
        code={jsonPretty}
        language="json"
        maxHeight="500px"
        searchQuery={searchQuery}
        isCurrentSearchMatch={isCurrentSearchMatch}
      />
    );
  }
  let html = renderMarkdownToHtml(text);
  if (searchQuery) {
    html = highlightTextInHtml(html, searchQuery, getHighlightClass(isCurrentSearchMatch ?? false));
  }
  return <div className={styles.markdown} dangerouslySetInnerHTML={{ __html: html }} />;
}
