// CF-418 spec: canonical TokenUsage + provider-keyed pricing.
// `calculateCost(provider, model, usage)` is the single cost arithmetic
// surface; provider-specific adjustments (Claude fast multiplier, web
// search add-on) live on the adapter via `calculateMessageCost`.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { type ProviderId } from './providers';
import {
  type TokenUsage,
  type PricingTable,
  calculateCost,
  normalizeClaudeUsage,
  setPricingTable,
  formatTokenCount,
  formatCost,
  getModelFamily,
  computeTokenSpeed,
  formatTokenSpeed,
  computeMessageTokenSpeed,
} from './tokenStats';
import { PRICING_FIXTURE } from '@/test/pricingFixture';

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
// CF-525: token speed = output tokens generated per second.
//
// These tests are the locked spec contract (Phase 3b). The numerator is output
// tokens only; the denominator is a duration in milliseconds (assistant
// generation time for the exact surfaces, predecessor-timestamp delta for the
// approximate per-message badge). Anything that can't divide cleanly is null,
// which the UI renders as "—" (cards/Trends) or omits (per-message badge).
// ---------------------------------------------------------------------------

describe('computeTokenSpeed', () => {
  it('computes output tokens per second for a normal duration', () => {
    // 1000 output tokens over 10s = 100 tok/s
    expect(computeTokenSpeed(1000, 10_000)).toBe(100);
  });

  it('computes fractional speeds', () => {
    expect(computeTokenSpeed(500, 2000)).toBe(250);
    expect(computeTokenSpeed(3, 1000)).toBe(3);
  });

  it('returns 0 when there are zero output tokens but real duration', () => {
    // Distinct from null: the rate is genuinely zero, not unmeasurable.
    expect(computeTokenSpeed(0, 5000)).toBe(0);
  });

  it('returns null when duration is zero (unmeasurable)', () => {
    expect(computeTokenSpeed(1000, 0)).toBeNull();
  });

  it('returns null when duration is negative (clock skew / back-stepping)', () => {
    expect(computeTokenSpeed(1000, -500)).toBeNull();
  });

  it('returns null when duration is NaN', () => {
    expect(computeTokenSpeed(1000, Number.NaN)).toBeNull();
  });

  it('passes large throughput through unrounded', () => {
    // Rounding/abbreviation is the formatter's job, not this function's.
    expect(computeTokenSpeed(50_000, 1000)).toBe(50_000);
  });
});

describe('formatTokenSpeed', () => {
  it('renders null as an em dash', () => {
    expect(formatTokenSpeed(null)).toBe('—');
  });

  it('renders a small integer speed', () => {
    expect(formatTokenSpeed(85)).toBe('85 tok/s');
  });

  it('renders zero as a real value (not a dash)', () => {
    expect(formatTokenSpeed(0)).toBe('0 tok/s');
  });

  it('rounds to the nearest whole token', () => {
    expect(formatTokenSpeed(85.7)).toBe('86 tok/s');
    expect(formatTokenSpeed(85.2)).toBe('85 tok/s');
  });

  it('abbreviates large speeds with a k-suffix (reusing formatTokenCount)', () => {
    expect(formatTokenSpeed(1500)).toBe('1.5k tok/s');
  });
});

describe('computeMessageTokenSpeed (approximate per-message)', () => {
  // duration = thisTimestamp - prevTimestamp (immediately preceding entry).
  const t0 = '2026-05-30T12:00:00.000Z';
  const t2s = '2026-05-30T12:00:02.000Z'; // +2s
  const t4s = '2026-05-30T12:00:04.000Z'; // +4s

  it('computes speed from the gap to the previous entry', () => {
    // 200 output tokens over a 2s gap = 100 tok/s
    expect(computeMessageTokenSpeed(200, t0, t2s)).toBe(100);
  });

  it('uses the immediately preceding timestamp as the gap start', () => {
    // 400 tokens over the 2s gap (t2s -> t4s) = 200 tok/s
    expect(computeMessageTokenSpeed(400, t2s, t4s)).toBe(200);
  });

  it('returns null when there is no predecessor (first entry)', () => {
    expect(computeMessageTokenSpeed(200, undefined, t0)).toBeNull();
  });

  it('returns null when output tokens are zero', () => {
    expect(computeMessageTokenSpeed(0, t0, t2s)).toBeNull();
  });

  it('returns null when timestamps are equal (streamed blocks share a stamp)', () => {
    expect(computeMessageTokenSpeed(200, t2s, t2s)).toBeNull();
  });

  it('returns null when the gap is negative (back-stepping timestamps)', () => {
    expect(computeMessageTokenSpeed(200, t4s, t0)).toBeNull();
  });

  it('returns null when a timestamp is unparseable', () => {
    expect(computeMessageTokenSpeed(200, 'not-a-date', t2s)).toBeNull();
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

  it('resolves the fable family with or without a date suffix', () => {
    expect(getModelFamily('claude-code', 'claude-fable-5')).toBe('fable-5');
    expect(getModelFamily('claude-code', 'claude-fable-5-20260601')).toBe('fable-5');
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
  return { input: 0, output: 0, cacheWrite: 0, cacheWrite1h: 0, cacheRead: 0, ...overrides };
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

  // rd9v: cache-creation split by ephemeral tier. cacheWrite = 5m count
  // (1.25x input), cacheWrite1h = 1h count (2x input). Numbers pinned to the
  // backend pricing_test.go cache-tier cases (opus-4-8: cacheWrite 6.25,
  // cacheWrite1h 10 per million).
  it('bills 1h cache writes at the cacheWrite1h rate', () => {
    // opus-4-8: cacheWrite1h=$10/M → 1M * $10/M = $10.00
    const cost = calculateCost(
      'claude-code',
      'claude-opus-4-8-20260515',
      usage({ cacheWrite1h: 1_000_000 }),
    );
    expect(cost).toBeCloseTo(10, 4);
  });

  it('bills mixed 5m + 1h cache writes at their respective rates', () => {
    // 400k * $6.25/M (5m) + 600k * $10/M (1h) = $2.50 + $6.00 = $8.50
    const cost = calculateCost(
      'claude-code',
      'claude-opus-4-8-20260515',
      usage({ cacheWrite: 400_000, cacheWrite1h: 600_000 }),
    );
    expect(cost).toBeCloseTo(8.5, 4);
  });

  it('falls back to the 5m rate for 1h tokens when cacheWrite1h is missing/0', () => {
    // Simulate a stale remote pricing doc with no cacheWrite1h field.
    const stale: PricingTable = {
      'claude-code': {
        'opus-4-8': { input: 5, output: 25, cacheWrite: 6.25, cacheWrite1h: 0, cacheRead: 0.5 },
      },
      codex: {},
      opencode: {},
    };
    setPricingTable(stale);
    try {
      const cost = calculateCost(
        'claude-code',
        'claude-opus-4-8-20260515',
        usage({ cacheWrite1h: 1_000_000 }),
      );
      // 1h priced at the 5m rate $6.25/M, NOT $0.
      expect(cost).toBeCloseTo(6.25, 4);
    } finally {
      setPricingTable(PRICING_FIXTURE); // restore the global fixture
    }
  });
});

// ---------------------------------------------------------------------------
// normalizeClaudeUsage — Claude wire shape → canonical TokenUsage, splitting
// cache-creation into 5m (cacheWrite) and 1h (cacheWrite1h) counts (rd9v).
// ---------------------------------------------------------------------------

describe('normalizeClaudeUsage', () => {
  it('splits the nested cache_creation object into 5m and 1h counts', () => {
    const u = normalizeClaudeUsage({
      input_tokens: 6,
      output_tokens: 221,
      cache_creation_input_tokens: 9726,
      cache_read_input_tokens: 17335,
      cache_creation: { ephemeral_5m_input_tokens: 1000, ephemeral_1h_input_tokens: 8726 },
    });
    expect(u.cacheWrite).toBe(1000);
    expect(u.cacheWrite1h).toBe(8726);
    expect(u.cacheRead).toBe(17335);
  });

  it('treats legacy usage (no cache_creation object) as all-5m', () => {
    const u = normalizeClaudeUsage({
      input_tokens: 100,
      output_tokens: 50,
      cache_creation_input_tokens: 200,
    });
    expect(u.cacheWrite).toBe(200);
    expect(u.cacheWrite1h).toBe(0);
  });

  it('defaults both cache-write counts to 0 when absent', () => {
    const u = normalizeClaudeUsage({ input_tokens: 10, output_tokens: 5 });
    expect(u.cacheWrite).toBe(0);
    expect(u.cacheWrite1h).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// getModelFamily parity — shared cross-language fixture (nrxr / 5x6e F7).
//
// The same testdata/model_family_parity.json is loaded by the backend
// (pricing_test.go) and here, so the two getModelFamily implementations can't
// silently drift. This side asserts the pinned `frontend` family key. The one
// documented divergence (the malformed `claude-opus-4a-5`) is encoded in the
// fixture as intentional — if any other id starts disagreeing, both sides fail.
// ---------------------------------------------------------------------------

interface ModelFamilyParityCase {
  provider: ProviderId;
  id: string;
  backend: string;
  frontend: string;
}

// Resolved from the Vitest root (the `frontend/` dir, per frontend/CLAUDE.md's
// "always run from frontend/") up to the repo-root testdata. Read via fs rather
// than import.meta.url because the jsdom test environment yields a non-file: URL.
const parityFixturePath = resolve(process.cwd(), '..', 'testdata', 'model_family_parity.json');
const parityFixture: { cases: ModelFamilyParityCase[] } = JSON.parse(
  readFileSync(parityFixturePath, 'utf-8'),
);

describe('getModelFamily parity fixture', () => {
  it('has cases', () => {
    expect(parityFixture.cases.length).toBeGreaterThan(0);
  });

  it.each(parityFixture.cases)(
    'returns $frontend for $id ($provider)',
    ({ provider, id, frontend }) => {
      expect(getModelFamily(provider, id)).toBe(frontend);
    },
  );
});
