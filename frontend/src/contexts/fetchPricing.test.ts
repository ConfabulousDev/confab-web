import { describe, it, expect, vi, afterEach } from 'vitest';
import { fetchPricing } from './fetchPricing';
import { calculateCost, setPricingTable, type PricingTable, type TokenUsage } from '@/utils/tokenStats';
import { PRICING_FIXTURE } from '@/test/pricingFixture';

const oneMillionInput: TokenUsage = { input: 1_000_000, output: 0, cacheWrite: 0, cacheRead: 0 };

function mockFetch(body: unknown, status = 200) {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(body), { status }));
}

afterEach(() => {
  vi.restoreAllMocks();
  setPricingTable(PRICING_FIXTURE); // restore the global fixture for sibling tests
});

describe('fetchPricing', () => {
  it('installs the table from the backend /api/v1/pricing response', async () => {
    const table: PricingTable = {
      'claude-code': { 'opus-4-7': { input: 7, output: 25, cacheWrite: 6.25, cacheRead: 0.5 } },
      codex: {},
    };
    mockFetch({ schema_version: 0, updated_at: '2026-06-01T00:00:00Z', pricing: table });

    await fetchPricing();

    // opus-4-7 input is now 7 → 1M input tokens = $7.00 (fixture has it at 5).
    expect(calculateCost('claude-code', 'claude-opus-4-7-20260301', oneMillionInput)).toBeCloseTo(7, 4);
  });

  it('leaves the existing table untouched when the response is not ok', async () => {
    mockFetch({ error: 'boom' }, 500);
    await fetchPricing();
    // Fixture still in effect: opus-4-7 input = 5 → $5.00.
    expect(calculateCost('claude-code', 'claude-opus-4-7-20260301', oneMillionInput)).toBeCloseTo(5, 4);
  });

  it('swallows network errors without throwing', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('network down'));
    await expect(fetchPricing()).resolves.toBeUndefined();
    // Fixture untouched.
    expect(calculateCost('claude-code', 'claude-opus-4-7-20260301', oneMillionInput)).toBeCloseTo(5, 4);
  });
});
