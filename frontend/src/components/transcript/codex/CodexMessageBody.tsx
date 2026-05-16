// Shared rendering path for Codex user + assistant message text.
//
// Mirrors `ContentBlock.tsx`'s text-block contract (CF-358): JSON-shaped text
// pretty-prints as a syntax-highlighted code block; everything else flows
// through the GFM markdown pipeline. Both message components consume this so
// the JSON-or-markdown fallback stays in one place.

import { renderMarkdownToHtml, tryParseAsJson } from '@/utils';
import { getHighlightClass, highlightTextInHtml } from '@/utils/highlightSearch';
import CodeBlock from '../CodeBlock';
import styles from './CodexMessage.module.css';

export interface CodexMessageBodyProps {
  text: string;
  /** CF-359: transcript search query — when set, matches inside the
   *  rendered markdown HTML are wrapped in `<mark>`. */
  searchQuery?: string;
  /** CF-359: marks this row as the active (n-of-N) match so the highlight
   *  uses the active-match CSS class. */
  isCurrentSearchMatch?: boolean;
}

export default function CodexMessageBody({
  text,
  searchQuery,
  isCurrentSearchMatch,
}: CodexMessageBodyProps) {
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
  return (
    <div
      className={styles.markdown}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
