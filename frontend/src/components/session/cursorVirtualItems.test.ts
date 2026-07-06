// 6h7m: unit tests for the Cursor virtual-item builder. Mirrors
// codexVirtualItems.test.tsx / claudeVirtualItems.test.ts. Cursor's
// timestamps are ESTIMATED (ce79) — interpolated, never real — so every
// separator this builder injects is flagged `estimated: true` (Decision 8),
// and when no item has a timestamp (session bounds unknown), zero dividers
// are ever synthesized (fail-safe, matching cursorTimelineSegments.ts's
// "no bogus times" stance).

import { describe, it, expect } from 'vitest';
import type { CursorRenderItem } from './cursorCategories';
import { buildVirtualItems } from './cursorVirtualItems';

function user(id: string, timestamp?: string): CursorRenderItem {
  return { kind: 'user', id, text: 'hi', timestamp };
}

function assistant(id: string, timestamp?: string): CursorRenderItem {
  return { kind: 'assistant', id, text: 'hello', timestamp };
}

describe('buildVirtualItems (Cursor)', () => {
  describe('time-gap separator', () => {
    it('injects a separator entry between items >5min apart', () => {
      const items = [user('u1', '2026-05-13T18:00:00Z'), assistant('a1', '2026-05-13T18:06:00Z')];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(3);
      expect(result[0]?.type).toBe('item');
      expect(result[1]?.type).toBe('separator');
      expect(result[2]?.type).toBe('item');
    });

    it('does not inject a separator for items <=5min apart', () => {
      const items = [user('u1', '2026-05-13T18:00:00Z'), assistant('a1', '2026-05-13T18:04:59Z')];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(2);
      expect(result.every((v) => v.type === 'item')).toBe(true);
    });

    it('does not inject a separator before the first item', () => {
      const result = buildVirtualItems([user('u1', '2026-05-13T18:00:00Z')]);
      expect(result).toHaveLength(1);
      expect(result[0]?.type).toBe('item');
    });
  });

  describe('day-boundary divider', () => {
    it('injects a separator across a calendar-day change even with a <5min gap', () => {
      const items = [
        user('u1', new Date(2026, 4, 13, 23, 59, 0).toISOString()),
        assistant('a1', new Date(2026, 4, 14, 0, 1, 0).toISOString()),
      ];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(3);
      expect(result[1]?.type).toBe('separator');
    });

    it('does not inject a separator for a same-day gap under 5min, even late at night', () => {
      const items = [
        user('u1', new Date(2026, 4, 13, 23, 50, 0).toISOString()),
        assistant('a1', new Date(2026, 4, 13, 23, 54, 0).toISOString()),
      ];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(2);
    });
  });

  describe('estimated flag (Decision 8)', () => {
    it('marks every injected separator as estimated', () => {
      const items = [user('u1', '2026-05-13T18:00:00Z'), assistant('a1', '2026-05-13T18:06:00Z')];
      const result = buildVirtualItems(items);
      const separator = result.find((v) => v.type === 'separator');
      expect(separator).toMatchObject({ type: 'separator', estimated: true });
    });
  });

  describe('unknown timestamp bounds (fail-safe)', () => {
    it('injects zero dividers when no item has a timestamp', () => {
      const items = [user('u1'), assistant('a1'), user('u2')];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(3);
      expect(result.every((v) => v.type === 'item')).toBe(true);
    });
  });

  it('item indices in VirtualItem.index reference the passed-in items array', () => {
    const items = [
      user('u1', '2026-05-13T18:00:00Z'),
      assistant('a1', '2026-05-13T18:00:01Z'),
      user('u2', '2026-05-13T18:06:01Z'), // triggers a separator before this
    ];
    const result = buildVirtualItems(items);
    const itemEntries = result.filter((v): v is Extract<typeof v, { type: 'item' }> => v.type === 'item');
    expect(itemEntries.map((v) => v.index)).toEqual([0, 1, 2]);
  });
});
