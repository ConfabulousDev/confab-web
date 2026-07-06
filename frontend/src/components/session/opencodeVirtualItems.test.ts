// 6h7m: unit tests for the OpenCode virtual-item builder. Mirrors
// codexVirtualItems.test.tsx / cursorVirtualItems.test.ts. OpenCode's
// `timeCreated` is a REAL, required epoch-ms field (info.time.created) — no
// estimation involved, so separators here are never flagged `estimated`.

import { describe, it, expect } from 'vitest';
import type { OpenCodeRenderItem } from './opencodeCategories';
import { buildVirtualItems } from './opencodeVirtualItems';

function user(id: string, timeCreated: number): OpenCodeRenderItem {
  return { kind: 'user', id, text: 'hi', timeCreated };
}

function assistant(id: string, timeCreated: number): OpenCodeRenderItem {
  return { kind: 'assistant', id, text: 'hello', timeCreated };
}

const T0 = new Date(2026, 4, 13, 18, 0, 0).getTime();

describe('buildVirtualItems (OpenCode)', () => {
  describe('time-gap separator', () => {
    it('injects a separator entry between items >5min apart', () => {
      const items = [user('u1', T0), assistant('a1', T0 + 6 * 60_000)];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(3);
      expect(result[0]?.type).toBe('item');
      expect(result[1]?.type).toBe('separator');
      expect(result[2]?.type).toBe('item');
    });

    it('does not inject a separator for items <=5min apart', () => {
      const items = [user('u1', T0), assistant('a1', T0 + 4 * 60_000 + 59_000)];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(2);
      expect(result.every((v) => v.type === 'item')).toBe(true);
    });

    it('does not inject a separator before the first item', () => {
      const result = buildVirtualItems([user('u1', T0)]);
      expect(result).toHaveLength(1);
      expect(result[0]?.type).toBe('item');
    });
  });

  describe('day-boundary divider', () => {
    it('injects a separator across a calendar-day change even with a <5min gap', () => {
      const may13_2359 = new Date(2026, 4, 13, 23, 59, 0).getTime();
      const may14_0001 = new Date(2026, 4, 14, 0, 1, 0).getTime();
      const items = [user('u1', may13_2359), assistant('a1', may14_0001)];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(3);
      expect(result[1]?.type).toBe('separator');
    });

    it('does not inject a separator for a same-day gap under 5min, even late at night', () => {
      const may13_2350 = new Date(2026, 4, 13, 23, 50, 0).getTime();
      const may13_2354 = new Date(2026, 4, 13, 23, 54, 0).getTime();
      const items = [user('u1', may13_2350), assistant('a1', may13_2354)];
      const result = buildVirtualItems(items);
      expect(result).toHaveLength(2);
    });

    it('separator label is the full weekday/month/day text on a day-boundary crossing', () => {
      const may13_2359 = new Date(2026, 4, 13, 23, 59, 0).getTime();
      const may14_0001 = new Date(2026, 4, 14, 0, 1, 0).getTime();
      const items = [user('u1', may13_2359), assistant('a1', may14_0001)];
      const result = buildVirtualItems(items);
      const separator = result.find((v) => v.type === 'separator');
      expect(separator).toBeDefined();
      if (separator?.type === 'separator') {
        expect(separator.label).toMatch(/\w+day/);
        expect(separator.label).toContain('May');
        expect(separator.label).toContain('14');
      }
    });
  });

  it('item indices in VirtualItem.index reference the original items array', () => {
    const items = [user('u1', T0), assistant('a1', T0 + 1000), user('u2', T0 + 6 * 60_000 + 1000)];
    const result = buildVirtualItems(items);
    const itemEntries = result.filter((v): v is Extract<typeof v, { type: 'item' }> => v.type === 'item');
    expect(itemEntries.map((v) => v.index)).toEqual([0, 1, 2]);
  });
});
