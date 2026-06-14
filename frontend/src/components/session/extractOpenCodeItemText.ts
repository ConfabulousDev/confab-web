// Plain-text search projection for one `OpenCodeRenderItem` (5p9j).
//
// Bridges `OpenCodeRenderItem` to the generic `useTranscriptSearch` hook,
// the same toolkit Claude and Codex use. The returned string is everything
// the user can see on the row — the hook lowercases it and `includes`-matches
// against it, so matches are found even in rows the virtualizer hasn't
// mounted yet.
//
// MUST stay a module-level export with a stable reference: passing it inline
// to `useTranscriptSearch` would churn the hook's `searchIndex` memo on every
// render. Mirrors `transcript/codex/extractCodexItemText.ts`.

import type { OpenCodeRenderItem } from './opencodeCategories';

/**
 * Stringify the raw line exactly as the unknown row renders it inside its
 * <pre>. Lives here (a non-component module) so both `OpenCodeUnknownItem` and
 * the search projection below index the same text — no drift, and no
 * react-refresh export warning from sharing it out of a component file.
 */
export function stringifyUnknownRaw(value: unknown): string {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function extractOpenCodeItemText(item: OpenCodeRenderItem): string {
  switch (item.kind) {
    case 'user':
      return item.text;
    case 'assistant':
      // Reasoning + visible text, in the order they render on the row.
      return [item.reasoning, item.text].filter(Boolean).join('\n');
    case 'tool':
      // Include tool I/O (decision 4): the user can read both the input and
      // the (collapsible) output, so both must be searchable.
      return [item.input, item.output].filter(Boolean).join('\n');
    case 'unknown':
      // Mirror exactly what `OpenCodeUnknownItem` displays inside its raw
      // <pre>, so the search index never drifts from the rendered text.
      return stringifyUnknownRaw(item.rawLine);
  }
}
