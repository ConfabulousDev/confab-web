// 18n2: Cursor transcript categories — flat user/assistant/tool filter state,
// count, and itemMatchesFilter. Mirrors opencodeCategories (no `unknown` row:
// the wire format has only two line shapes, both fully handled).

import { describe, expect, it } from 'vitest';
import {
  DEFAULT_CURSOR_FILTER_STATE,
  countCursorCategories,
  cursorItemMatchesFilter,
  type CursorRenderItem,
} from './cursorCategories';

const items: CursorRenderItem[] = [
  { kind: 'user', id: '0', text: 'hi' },
  { kind: 'assistant', id: '1', text: 'hello' },
  { kind: 'tool', id: '1-0', toolName: 'Read', input: 'foo.go' },
  { kind: 'tool', id: '1-1', toolName: 'Grep', input: 'pattern' },
];

describe('countCursorCategories', () => {
  it('tallies each kind', () => {
    expect(countCursorCategories(items)).toEqual({ user: 1, assistant: 1, tool: 2 });
  });

  it('returns zeros for an empty list', () => {
    expect(countCursorCategories([])).toEqual({ user: 0, assistant: 0, tool: 0 });
  });
});

describe('cursorItemMatchesFilter', () => {
  it('shows every kind under the default (all-visible) state', () => {
    for (const item of items) {
      expect(cursorItemMatchesFilter(item, DEFAULT_CURSOR_FILTER_STATE)).toBe(true);
    }
  });

  it('hides a kind when its filter boolean is false', () => {
    const state = { ...DEFAULT_CURSOR_FILTER_STATE, tool: false };
    const tool = items[2]!;
    const user = items[0]!;
    expect(cursorItemMatchesFilter(tool, state)).toBe(false);
    expect(cursorItemMatchesFilter(user, state)).toBe(true);
  });
});
