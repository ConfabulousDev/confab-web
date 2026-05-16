// React-JSX companion to `highlightSearch.ts`.
//
// Separate file because `highlightSearch.ts` stays `.ts` (its callers — the
// hook, parsers — must not pull React into their import graph). Anything that
// returns JSX lives here so plain-text utilities can stay framework-free.

import type { ReactNode } from 'react';
import { getHighlightClass, splitTextByQuery } from './highlightSearch';

/**
 * Render plain text with search-query matches wrapped in `<mark>` elements.
 * For React-text-node surfaces (commands, file paths, chips, divider labels)
 * where `dangerouslySetInnerHTML` would be overkill. Mirrors the thinking-block
 * pattern in `ContentBlock.tsx`.
 *
 * When `query` is falsy, returns the original text unchanged so callers can
 * unconditionally pipe through this helper without a guard.
 */
export function renderTextWithHighlight(
  text: string,
  query: string | undefined,
  isActiveMatch: boolean | undefined,
): ReactNode {
  if (!query) return text;
  const className = getHighlightClass(isActiveMatch ?? false);
  return splitTextByQuery(text, query).map((seg, i) =>
    typeof seg === 'string' ? seg : <mark key={i} className={className}>{seg.match}</mark>,
  );
}
