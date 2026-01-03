import { describe, it, expect } from 'vitest';
import { cardRegistry, getOrderedCards } from './registry';

describe('cardRegistry', () => {
  it('contains expected cards', () => {
    const keys = cardRegistry.map((c) => c.key);

    expect(keys).toContain('tokens');
    expect(keys).toContain('session');
    expect(keys).toContain('conversation');
    expect(keys).toContain('code_activity');
    expect(keys).toContain('tools');
    expect(keys).toContain('agents_and_skills');
    expect(keys).toContain('redactions');
    // Note: cost is now part of tokens card, compaction is part of session card
    // Note: agents and skills are now combined into a single card
    expect(keys).toHaveLength(7);
  });

  it('has unique keys', () => {
    const keys = cardRegistry.map((c) => c.key);
    const uniqueKeys = new Set(keys);

    expect(uniqueKeys.size).toBe(keys.length);
  });

  it('has unique order values', () => {
    const orders = cardRegistry.map((c) => c.order);
    const uniqueOrders = new Set(orders);

    expect(uniqueOrders.size).toBe(orders.length);
  });

  it('all cards have required properties', () => {
    for (const card of cardRegistry) {
      expect(card.key).toBeDefined();
      expect(card.title).toBeDefined();
      expect(card.component).toBeDefined();
      expect(typeof card.order).toBe('number');
    }
  });
});

describe('getOrderedCards', () => {
  it('returns cards sorted by order', () => {
    const ordered = getOrderedCards();

    for (let i = 1; i < ordered.length; i++) {
      expect(ordered[i]!.order).toBeGreaterThan(ordered[i - 1]!.order);
    }
  });

  it('returns a new array (does not mutate registry)', () => {
    const ordered = getOrderedCards();

    expect(ordered).not.toBe(cardRegistry);
    expect(ordered).toEqual(cardRegistry); // Same content initially
  });

  it('returns all cards in expected order', () => {
    const ordered = getOrderedCards();
    const keys = ordered.map((c) => c.key);

    expect(keys).toEqual(['tokens', 'session', 'conversation', 'code_activity', 'tools', 'agents_and_skills', 'redactions']);
  });
});
