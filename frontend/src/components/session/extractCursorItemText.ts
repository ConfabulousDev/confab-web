// Plain-text search projection for one `CursorRenderItem` (18n2).
//
// Bridges `CursorRenderItem` to the generic `useTranscriptSearch` hook (the
// same toolkit Claude / Codex / OpenCode use). The returned string is
// everything the user can see on the row — the hook lowercases it and
// `includes`-matches against it, so matches are found even in rows the
// virtualizer hasn't mounted yet.
//
// MUST stay a module-level export with a stable reference: passing it inline to
// `useTranscriptSearch` would churn the hook's `searchIndex` memo on every
// render. Mirrors extractOpenCodeItemText.
//
// Cursor records tool INPUTS only (no output), so a tool row's searchable text
// is its name + input summary.

import type { CursorRenderItem } from './cursorCategories';

export function extractCursorItemText(item: CursorRenderItem): string {
  switch (item.kind) {
    case 'user':
    case 'assistant':
      return item.text;
    case 'tool':
      return [item.toolName, item.input].filter(Boolean).join('\n');
  }
}
