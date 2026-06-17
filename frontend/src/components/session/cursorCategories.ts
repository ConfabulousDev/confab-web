// Cursor transcript render items + flat filter categories (18n2).
//
// Cursor's wire format has two line shapes (message / turn_ended) and two
// content-block types (text / tool_use). turn_ended rows are dropped during
// normalization, so the render stream is exactly user / assistant / tool — a
// flat category set mirroring opencodeCategories (minus `unknown`: both wire
// shapes are fully handled, so there is no residual catch-all kind).
//
// Render items carry NO timeCreated / model / usage / cost — Cursor lines have
// none of those fields. Tool items carry only the call (name + a one-line input
// summary); there is no tool output to show (Cursor records inputs only).

export type CursorCategory = 'user' | 'assistant' | 'tool';

export type CursorRenderItem =
  | { kind: 'user'; id: string; text: string }
  | { kind: 'assistant'; id: string; text: string }
  | { kind: 'tool'; id: string; toolName: string; input?: string };

export type CursorFilterState = {
  user: boolean;
  assistant: boolean;
  tool: boolean;
};

export type CursorHierarchicalCounts = {
  user: number;
  assistant: number;
  tool: number;
};

export const DEFAULT_CURSOR_FILTER_STATE: CursorFilterState = {
  user: true,
  assistant: true,
  tool: true,
};

export function countCursorCategories(items: CursorRenderItem[]): CursorHierarchicalCounts {
  const counts: CursorHierarchicalCounts = { user: 0, assistant: 0, tool: 0 };
  for (const item of items) {
    counts[item.kind]++;
  }
  return counts;
}

export function cursorItemMatchesFilter(
  item: CursorRenderItem,
  state: CursorFilterState,
): boolean {
  return state[item.kind];
}
