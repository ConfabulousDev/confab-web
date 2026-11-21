import { describe, it, expect } from 'vitest';
import {
  buildScatterSeries,
  buildShareRows,
  insightsParams,
  logAxis,
  timeTicks,
  providerMarker,
  shareProviders,
} from './sessionInsights';
import { fixturePoint, fixtureShare } from './sessionInsightsFixtures';

describe('insightsParams', () => {
  const now = new Date('2026-03-31T12:00:00Z');

  it('computes from as now minus the preset days', () => {
    expect(insightsParams({ range: '7d', providers: [] }, now)).toEqual({
      providers: [],
      userId: undefined,
      from: '2026-03-24T12:00:00.000Z',
    });
  });

  it('sends all=true and no from for the all-time preset', () => {
    const params = insightsParams({ range: 'all', providers: ['codex'], userId: 4 }, now);
    expect(params).toEqual({ providers: ['codex'], userId: 4, all: true });
    expect(params.from).toBeUndefined();
  });
});

describe('buildShareRows', () => {
  it('zero-fills every day between from and to', () => {
    const rows = buildShareRows(fixtureShare, 'count');
    expect(rows).toHaveLength(7);
    expect(rows[0]!.start).toBe(Date.parse('2026-03-01T00:00:00Z'));
    expect(rows[0]!.total).toBe(0);
    expect(rows[0]!['claude-code']).toBe(0);
    expect(rows[1]!['claude-code']).toBe(6);
    expect(rows[1]!.total).toBe(8);
  });

  it('turns counts into per-bucket percentages in percent mode, 0 for empty buckets', () => {
    const rows = buildShareRows(fixtureShare, 'percent');
    expect(rows[1]!['claude-code']).toBeCloseTo(75);
    expect(rows[1]!.codex).toBeCloseTo(25);
    expect(rows[0]!['claude-code']).toBe(0);
  });

  it('starts weekly buckets on the Monday on or before from', () => {
    const rows = buildShareRows(
      { ...fixtureShare, bucket_size: 'week', from: '2026-03-04T15:00:00Z', to: '2026-03-20T00:00:00Z', buckets: [] },
      'count',
    );
    expect(rows.map((r) => new Date(r.start).toISOString().slice(0, 10))).toEqual([
      '2026-03-02',
      '2026-03-09',
      '2026-03-16',
    ]);
  });

  it('steps monthly buckets by calendar month', () => {
    const rows = buildShareRows(
      { ...fixtureShare, bucket_size: 'month', from: '2025-11-20T00:00:00Z', to: '2026-02-01T00:00:00Z', buckets: [] },
      'count',
    );
    expect(rows.map((r) => new Date(r.start).toISOString().slice(0, 10))).toEqual([
      '2025-11-01',
      '2025-12-01',
      '2026-01-01',
    ]);
  });

  it('returns no rows for an empty range', () => {
    expect(buildShareRows({ ...fixtureShare, from: fixtureShare.to, buckets: [], totals: {} }, 'count')).toEqual([]);
  });
});

describe('shareProviders', () => {
  it('orders present providers by the registry, unknown values last, zero totals dropped', () => {
    expect(shareProviders({ ...fixtureShare, totals: { zeta: 1, codex: 2, 'claude-code': 1, cursor: 0 } })).toEqual([
      'claude-code',
      'codex',
      'zeta',
    ]);
  });
});

describe('buildScatterSeries', () => {
  const points = [
    fixturePoint({ id: 'a', provider: 'codex', lines: 10, cost_usd: null }),
    fixturePoint({ id: 'b', provider: 'claude-code', lines: 20, cost_usd: 0 }),
    fixturePoint({ id: 'c', provider: 'claude-code', lines: 30, cost_usd: 2.5 }),
  ];

  it('groups by provider in registry order with y from the selected metric', () => {
    const { series, excluded } = buildScatterSeries(points, 'lines');
    expect(series.map((s) => s.provider)).toEqual(['claude-code', 'codex']);
    expect(series[0]!.data.map((d) => d.y)).toEqual([20, 30]);
    expect(excluded).toBe(0);
  });

  it('switching the metric changes y and excludes null or non-positive values', () => {
    const { series, excluded } = buildScatterSeries(points, 'cost_usd');
    expect(series).toHaveLength(1);
    expect(series[0]!.data.map((d) => d.point.id)).toEqual(['c']);
    expect(series[0]!.data[0]!.y).toBe(2.5);
    expect(excluded).toBe(2);
  });
});

describe('providerMarker', () => {
  it('gives each known provider a distinct shape and unknown providers a fallback', () => {
    const shapes = ['claude-code', 'codex', 'opencode', 'cursor'].map(providerMarker);
    expect(new Set(shapes).size).toBe(4);
    expect(providerMarker('future-agent')).toBe('cross');
  });
});

describe('logAxis', () => {
  it('snaps the domain outward to whole decades so nothing is clipped', () => {
    const { domain, ticks } = logAxis([37, 1_850]);
    expect(domain).toEqual([10, 10_000]);
    expect(ticks[0]).toBe(10);
    expect(ticks.at(-1)).toBe(10_000);
    expect(ticks).toContain(2_000);
  });

  it('keeps only decade ticks over a wide span and widens a single-decade span', () => {
    expect(logAxis([0.01, 5_000_000]).ticks.every((t) => Number.isInteger(Math.round(Math.log10(t) * 1e9) / 1e9))).toBe(true);
    expect(logAxis([100, 100]).domain).toEqual([100, 1_000]);
  });
});

describe('timeTicks', () => {
  it('spaces ticks evenly from min to max inclusive', () => {
    expect(timeTicks(0, 100, 5)).toEqual([0, 25, 50, 75, 100]);
    expect(timeTicks(42, 42)).toEqual([42]);
  });
});
