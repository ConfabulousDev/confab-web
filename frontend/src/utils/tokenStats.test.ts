// CF-418 spec: canonical TokenUsage + provider-keyed pricing.
// `calculateCost(provider, model, usage)` is the single cost arithmetic
// surface; provider-specific adjustments (Claude fast multiplier, web
// search add-on) live on the adapter via `calculateMessageCost`.

import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import {
  type TokenUsage,
  calculateCost,
  formatTokenCount,
  formatCost,
  getModelFamily,
} from './tokenStats';

// ---------------------------------------------------------------------------
// Format helpers — behavior preserved from pre-refactor code.
// ---------------------------------------------------------------------------

describe('formatTokenCount', () => {
  it('returns raw number for values under 1000', () => {
    expect(formatTokenCount(0)).toBe('0');
    expect(formatTokenCount(1)).toBe('1');
    expect(formatTokenCount(999)).toBe('999');
  });

  it('formats values >= 1000 with k suffix', () => {
    expect(formatTokenCount(1000)).toBe('1.0k');
    expect(formatTokenCount(1500)).toBe('1.5k');
    expect(formatTokenCount(145892)).toBe('145.9k');
  });

  it('formats values >= 1M with M suffix', () => {
    expect(formatTokenCount(1_000_000)).toBe('1.0M');
    expect(formatTokenCount(1_500_000)).toBe('1.5M');
  });

  it('formats values >= 1B with B suffix', () => {
    expect(formatTokenCount(1_000_000_000)).toBe('1.0B');
    expect(formatTokenCount(1_500_000_000)).toBe('1.5B');
  });
});

describe('formatCost', () => {
  it('formats costs with dollar sign and 2 decimals', () => {
    expect(formatCost(0.5)).toBe('$0.50');
    expect(formatCost(4.23)).toBe('$4.23');
    expect(formatCost(123.45)).toBe('$123.45');
  });

  it('shows $0.00 for exactly zero', () => {
    expect(formatCost(0)).toBe('$0.00');
  });

  it('shows <$0.01 for tiny non-zero costs', () => {
    expect(formatCost(0.001)).toBe('<$0.01');
    expect(formatCost(0.009)).toBe('<$0.01');
  });
});

// ---------------------------------------------------------------------------
// getModelFamily — now takes provider explicitly. No `isOpenAIModel` sniff.
// ---------------------------------------------------------------------------

describe('getModelFamily', () => {
  it('strips claude- prefix and date suffix for Claude models', () => {
    expect(getModelFamily('claude-code', 'claude-opus-4-7-20260301')).toBe('opus-4-7');
    expect(getModelFamily('claude-code', 'claude-sonnet-4-6-20260201')).toBe('sonnet-4-6');
    expect(getModelFamily('claude-code', 'claude-haiku-3-5-20241022')).toBe('haiku-3-5');
  });

  it('strips OpenAI pinned-snapshot date suffix for Codex models', () => {
    expect(getModelFamily('codex', 'gpt-5-2026-05-01')).toBe('gpt-5');
    expect(getModelFamily('codex', 'gpt-4o-mini-2024-11-20')).toBe('gpt-4o-mini');
  });

  it('passes through Codex models without date suffix', () => {
    expect(getModelFamily('codex', 'gpt-5.5')).toBe('gpt-5.5');
    expect(getModelFamily('codex', 'gpt-5-mini')).toBe('gpt-5-mini');
    expect(getModelFamily('codex', 'o3-mini')).toBe('o3-mini');
  });

  it('throws when provider is unknown', () => {
    expect(() =>
      // @ts-expect-error — exercising the unknown-provider error path.
      getModelFamily('cursor', 'whatever'),
    ).toThrow();
  });
});

// ---------------------------------------------------------------------------
// calculateCost — single arithmetic surface. Reproduces pre-refactor totals
// when called with canonical TokenUsage derived from the wire shape.
// ---------------------------------------------------------------------------

function usage(overrides: Partial<TokenUsage> = {}): TokenUsage {
  return { input: 0, output: 0, cacheWrite: 0, cacheRead: 0, ...overrides };
}

describe('calculateCost', () => {
  let warn: ReturnType<typeof vi.spyOn>;
  beforeEach(() => {
    warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
  });
  afterEach(() => {
    warn.mockRestore();
  });

  it('bills Claude sonnet-4 input + output at documented rates', () => {
    // sonnet-4: input=$3/M, output=$15/M
    // 100k in * $3/M = $0.30, 10k out * $15/M = $0.15 → $0.45
    const cost = calculateCost(
      'claude-code',
      'claude-sonnet-4-20250514',
      usage({ input: 100_000, output: 10_000 }),
    );
    expect(cost).toBeCloseTo(0.45, 4);
  });

  it('bills Claude cache write and cache read separately', () => {
    // sonnet-4-6: input=$3, output=$15, cacheWrite=$3.75, cacheRead=$0.30
    // 50k in * $3/M = $0.15
    // 5k out * $15/M = $0.075
    // 20k cacheWrite * $3.75/M = $0.075
    // 10k cacheRead * $0.30/M = $0.003
    // total = $0.303
    const cost = calculateCost(
      'claude-code',
      'claude-sonnet-4-6-20260201',
      usage({ input: 50_000, output: 5_000, cacheWrite: 20_000, cacheRead: 10_000 }),
    );
    expect(cost).toBeCloseTo(0.303, 4);
  });

  it('bills gpt-5 input + output at documented rates', () => {
    // gpt-5: input=$1.25/M, output=$10/M
    // 1M in * $1.25/M = $1.25, 100k out * $10/M = $1.00 → $2.25
    const cost = calculateCost(
      'codex',
      'gpt-5',
      usage({ input: 1_000_000, output: 100_000 }),
    );
    expect(cost).toBeCloseTo(2.25, 4);
  });

  it('bills cached Codex input at the cache-read rate, never at the input rate', () => {
    // Caller already split: input=uncached, cacheRead=cached. No subtraction
    // inside calculateCost — that's the parse-layer's responsibility.
    // gpt-5: input=$1.25/M, cacheRead=$0.125/M
    // 800k uncached * $1.25/M = $1.00
    // 200k cacheRead * $0.125/M = $0.025
    // total = $1.025
    const cost = calculateCost(
      'codex',
      'gpt-5',
      usage({ input: 800_000, cacheRead: 200_000 }),
    );
    expect(cost).toBeCloseTo(1.025, 4);
  });

  it('treats Codex cacheWrite as free (rate is 0 in the table)', () => {
    // Any value of cacheWrite contributes $0 for Codex models.
    const a = calculateCost('codex', 'gpt-5', usage({ input: 100_000 }));
    const b = calculateCost(
      'codex',
      'gpt-5',
      usage({ input: 100_000, cacheWrite: 999_999_999 }),
    );
    expect(a).toBeCloseTo(b, 6);
  });

  it('returns 0 for zero usage', () => {
    expect(calculateCost('claude-code', 'claude-sonnet-4-20250514', usage())).toBe(0);
    expect(calculateCost('codex', 'gpt-5', usage())).toBe(0);
  });

  it('warns and returns 0 for unknown model within a known provider', () => {
    const cost = calculateCost(
      'claude-code',
      'claude-unknown-model',
      usage({ input: 1_000_000 }),
    );
    expect(cost).toBe(0);
    expect(warn).toHaveBeenCalled();
  });

  it('throws when provider is unknown', () => {
    expect(() =>
      // @ts-expect-error — exercising the unknown-provider error path.
      calculateCost('cursor', 'whatever', usage({ input: 100 })),
    ).toThrow();
  });

  it('does NOT apply any fast multiplier or server-tool add-on', () => {
    // calculateCost is pure arithmetic over the canonical shape. Fast-mode
    // and web-search-cost adjustments live on the Claude adapter's
    // calculateMessageCost — not here.
    const cost = calculateCost(
      'claude-code',
      'claude-opus-4-6-20260201',
      usage({ input: 1_000_000, output: 100_000 }),
    );
    // opus-4-6: input=$5, output=$25 → 1M*5 + 100k*25 = $5 + $2.50 = $7.50
    expect(cost).toBeCloseTo(7.5, 4);
  });
});
