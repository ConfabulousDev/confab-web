// CF-417 spec: registry maps every canonical provider to an adapter whose
// `id` matches, and throws cleanly on unknown providers.

import { describe, expect, it } from 'vitest';
import { PROVIDER_VALUES } from '@/utils/providers';
import { getAdapter, isTokensMeasurable } from './registry';

describe('provider registry', () => {
  it.each(PROVIDER_VALUES)('returns adapter whose id matches "%s"', (id) => {
    const adapter = getAdapter(id);
    expect(adapter.id).toBe(id);
  });

  it('every PROVIDER_VALUES entry resolves to a distinct adapter (drift guard)', () => {
    const adapters = PROVIDER_VALUES.map((id) => getAdapter(id));
    const ids = new Set(adapters.map((a) => a.id));
    expect(ids.size).toBe(PROVIDER_VALUES.length);
  });

  it('normalizes "Claude Code" to claude-code', () => {
    expect(getAdapter('Claude Code').id).toBe('claude-code');
  });

  it('normalizes mixed-case "Codex" to codex', () => {
    expect(getAdapter('Codex').id).toBe('codex');
  });

  it('throws on unknown provider', () => {
    expect(() => getAdapter('windsurf')).toThrowError(/no adapter registered/);
  });

  it('throws on empty string', () => {
    expect(() => getAdapter('')).toThrowError();
  });

  describe('isTokensMeasurable (st5f)', () => {
    it('returns false for cursor', () => {
      expect(isTokensMeasurable('cursor')).toBe(false);
    });

    it.each(['claude-code', 'codex', 'opencode'] as const)(
      'returns true for measurable provider "%s"',
      (id) => {
        expect(isTokensMeasurable(id)).toBe(true);
      },
    );

    it('returns true for unknown provider ids (defaults measurable)', () => {
      expect(isTokensMeasurable('gemini')).toBe(true);
    });
  });
});
