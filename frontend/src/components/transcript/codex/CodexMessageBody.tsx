// Shared rendering path for Codex user + assistant message text.
//
// Mirrors `ContentBlock.tsx`'s text-block contract (CF-358): JSON-shaped text
// pretty-prints as a syntax-highlighted code block; everything else flows
// through the GFM markdown pipeline. Both message components consume this so
// the JSON-or-markdown fallback stays in one place.

import { renderMarkdownToHtml, tryParseAsJson } from '@/utils';
import CodeBlock from '../CodeBlock';
import styles from './CodexMessage.module.css';

export interface CodexMessageBodyProps {
  text: string;
}

export default function CodexMessageBody({ text }: CodexMessageBodyProps) {
  const jsonPretty = tryParseAsJson(text);
  if (jsonPretty) {
    return <CodeBlock code={jsonPretty} language="json" maxHeight="500px" />;
  }
  return (
    <div
      className={styles.markdown}
      dangerouslySetInnerHTML={{ __html: renderMarkdownToHtml(text) }}
    />
  );
}
