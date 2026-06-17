// a9gr: pure helpers for the Cursor transcript pane's per-row action cluster
// (shared `RowActions`). Extracted from `CursorTranscriptPane` so the
// same-kind skip-nav neighbor math and the per-row copy payload can be
// unit-tested without spinning up the virtualizer.

import type { CursorRenderItem } from './cursorCategories';

/** Same-kind skip-nav neighbor maps, keyed by `filteredItems` index. */
export interface CursorRowNav {
  /** index → index of the next row of the SAME kind (user/assistant/tool). */
  nextOfSameKind: Map<number, number>;
  /** index → index of the previous row of the SAME kind. */
  prevOfSameKind: Map<number, number>;
}

/**
 * Build next-/prev-of-same-kind skip-nav maps over the (filtered) render
 * items. Mirrors Codex's `CodexMessageTimeline` same-kind chain logic, but
 * Cursor's kind set is flat (user / assistant / tool) with no per-tool split.
 * A row with no same-kind neighbor in a direction simply has no entry in that
 * map — the caller hides the corresponding button at the ends of a chain.
 */
export function buildCursorRowNav(items: CursorRenderItem[]): CursorRowNav {
  const next = new Map<number, number>();
  const prev = new Map<number, number>();
  const lastSeenByKind = new Map<string, number>();
  items.forEach((item, idx) => {
    const prevIdx = lastSeenByKind.get(item.kind);
    if (prevIdx !== undefined) {
      next.set(prevIdx, idx);
      prev.set(idx, prevIdx);
    }
    lastSeenByKind.set(item.kind, idx);
  });
  return { nextOfSameKind: next, prevOfSameKind: prev };
}

/** Human-readable kind label for the row's aria-label/title (skip + actions). */
export function cursorRowKindLabel(item: CursorRenderItem): string {
  switch (item.kind) {
    case 'user':
      return 'user prompt';
    case 'assistant':
      return 'assistant message';
    case 'tool':
      return 'tool call';
  }
}

/**
 * The raw text copied by a row's copy-text button (D3). User/assistant rows
 * copy their narrative source (`item.text`, the paste-friendly markdown
 * source, NOT rendered HTML); tool rows copy the input summary (`item.input`).
 * Returns `undefined` when there is nothing to copy, so `RowActions` hides the
 * button (it also guards against whitespace-only values).
 */
export function buildCursorRowCopyText(item: CursorRenderItem): string | undefined {
  switch (item.kind) {
    case 'user':
    case 'assistant':
      return item.text || undefined;
    case 'tool':
      return item.input || undefined;
  }
}
