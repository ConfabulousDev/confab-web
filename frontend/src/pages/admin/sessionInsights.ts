// Pure helpers for the admin Session Insights tab (c3ts): range presets, the
// provider-share zero-fill and percent transform, and the size-scatter series.
// Kept out of the component so the chart data is testable without recharts.

import type { ProviderShareResponse, SessionPoint } from '@/schemas/api';
import type { SessionInsightsParams } from '@/services/api';
import { PROVIDER_VALUES } from '@/utils/providers';
import { formatCostCompact, formatTokenCount } from '@/utils/tokenStats';
import { formatDuration } from '@/utils/formatting';

export const RANGE_PRESETS = [
  { id: '7d', label: '7d', days: 7 },
  { id: '30d', label: '30d', days: 30 },
  { id: '90d', label: '90d', days: 90 },
  { id: '1y', label: '1y', days: 365 },
  { id: 'all', label: 'All', days: null },
] as const;
export type RangePreset = (typeof RANGE_PRESETS)[number]['id'];

export interface SessionInsightsFilters {
  range: RangePreset;
  providers: string[];
  userId?: number;
}

/** Request params for a filter set, with `from` computed against `now`. */
export function insightsParams(filters: SessionInsightsFilters, now: Date = new Date()): SessionInsightsParams {
  const preset = RANGE_PRESETS.find((p) => p.id === filters.range) ?? RANGE_PRESETS[1];
  const base = { providers: filters.providers, userId: filters.userId };
  if (preset.days === null) return { ...base, all: true };
  return { ...base, from: new Date(now.getTime() - preset.days * 86_400_000).toISOString() };
}

const DAY_MS = 86_400_000;

/** Truncates to the bucket start in UTC: midnight, Monday, or the 1st. */
function bucketStart(ms: number, size: ProviderShareResponse['bucket_size']): number {
  const d = new Date(ms);
  const midnight = Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate());
  if (size === 'day') return midnight;
  if (size === 'week') return midnight - ((d.getUTCDay() + 6) % 7) * DAY_MS;
  return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1);
}

function nextBucket(ms: number, size: ProviderShareResponse['bucket_size']): number {
  if (size === 'day') return ms + DAY_MS;
  if (size === 'week') return ms + 7 * DAY_MS;
  const d = new Date(ms);
  return Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 1);
}

export interface ShareRow {
  start: number;
  total: number;
  /** Per-provider value: a count, or a 0–100 share in percent mode. */
  [provider: string]: number;
}

function isKnownProvider(p: string): boolean {
  return PROVIDER_VALUES.some((v) => v === p);
}

/** Providers present in the totals, in registry order (unknown values last). */
export function shareProviders(share: ProviderShareResponse): string[] {
  const present = Object.keys(share.totals).filter((p) => (share.totals[p] ?? 0) > 0);
  const known = PROVIDER_VALUES.filter((p) => present.includes(p));
  const unknown = present.filter((p) => !isKnownProvider(p)).sort();
  return [...known, ...unknown];
}

/**
 * One row per bucket from `from` to `to`, zero-filling the buckets the server
 * omitted. In percent mode each provider value is its share of that bucket.
 */
export function buildShareRows(share: ProviderShareResponse, mode: 'count' | 'percent'): ShareRow[] {
  const providers = shareProviders(share);
  const byStart = new Map(share.buckets.map((b) => [Date.parse(b.start), b.counts]));
  const end = Date.parse(share.to);
  const rows: ShareRow[] = [];
  // Cap guards against a malformed range producing an unbounded loop.
  for (let t = bucketStart(Date.parse(share.from), share.bucket_size); t < end && rows.length < 2000; t = nextBucket(t, share.bucket_size)) {
    const counts = byStart.get(t) ?? {};
    const total = providers.reduce((sum, p) => sum + (counts[p] ?? 0), 0);
    const row: ShareRow = { start: t, total };
    for (const p of providers) {
      const n = counts[p] ?? 0;
      row[p] = mode === 'percent' ? (total > 0 ? (n / total) * 100 : 0) : n;
    }
    rows.push(row);
  }
  return rows;
}

export function formatBucketLabel(ms: number, size: ProviderShareResponse['bucket_size']): string {
  const opts: Intl.DateTimeFormatOptions =
    size === 'month' ? { month: 'short', year: 'numeric', timeZone: 'UTC' } : { month: 'short', day: 'numeric', timeZone: 'UTC' };
  return new Date(ms).toLocaleDateString(undefined, opts);
}

export type SizeMetric = 'lines' | 'tokens' | 'cost_usd' | 'user_turns' | 'duration_ms';

export const SIZE_METRICS: { id: SizeMetric; label: string; noun: string; format: (v: number) => string }[] = [
  { id: 'lines', label: 'Transcript lines', noun: 'transcript lines', format: (v) => v.toLocaleString() },
  { id: 'tokens', label: 'Tokens (input + output)', noun: 'token data', format: formatTokenCount },
  { id: 'cost_usd', label: 'Estimated cost', noun: 'cost data', format: formatCostCompact },
  { id: 'user_turns', label: 'User turns', noun: 'turn data', format: (v) => v.toLocaleString() },
  { id: 'duration_ms', label: 'Duration', noun: 'duration data', format: (v) => formatDuration(v) },
];

export function sizeMetric(id: SizeMetric) {
  return SIZE_METRICS.find((m) => m.id === id) ?? SIZE_METRICS[0]!;
}

export interface ScatterDatum {
  x: number;
  y: number;
  point: SessionPoint;
}

interface ScatterSeries {
  provider: string;
  data: ScatterDatum[];
}

/**
 * Groups points into one series per provider (registry order) for the selected
 * metric. Points with a null or non-positive value can't sit on a log axis, so
 * they are left out and counted in `excluded`.
 */
export function buildScatterSeries(points: SessionPoint[], metric: SizeMetric): { series: ScatterSeries[]; excluded: number } {
  const byProvider = new Map<string, ScatterDatum[]>();
  let excluded = 0;
  for (const point of points) {
    const y = point[metric];
    if (y === null || y <= 0) {
      excluded++;
      continue;
    }
    const list = byProvider.get(point.provider) ?? [];
    list.push({ x: Date.parse(point.first_seen), y, point });
    byProvider.set(point.provider, list);
  }
  const order = [
    ...PROVIDER_VALUES.filter((p) => byProvider.has(p)),
    ...[...byProvider.keys()].filter((p) => !isKnownProvider(p)).sort(),
  ];
  return { series: order.map((provider) => ({ provider, data: byProvider.get(provider)! })), excluded };
}

/**
 * Marker shape per provider: secondary encoding so identity never rests on
 * color alone (OpenCode's and Cursor's registry grays are near-identical).
 */
export type MarkerShape = 'circle' | 'square' | 'triangle' | 'diamond' | 'cross';

export function providerMarker(provider: string): MarkerShape {
  switch (provider) {
    case 'claude-code':
      return 'circle';
    case 'codex':
      return 'square';
    case 'opencode':
      return 'triangle';
    case 'cursor':
      return 'diamond';
    default:
      return 'cross';
  }
}

/**
 * Log-axis bounds snapped outward to whole decades, with ticks at 1×, 2× and 5×
 * each decade (only 1× once the span exceeds four decades), so no positive
 * value is clipped. Values must be positive.
 */
export function logAxis(values: number[]): { domain: [number, number]; ticks: number[] } {
  if (values.length === 0) return { domain: [1, 10], ticks: [1, 10] };
  const lo = Math.floor(Math.log10(Math.min(...values)));
  let hi = Math.ceil(Math.log10(Math.max(...values)));
  if (hi === lo) hi = lo + 1;
  const steps = hi - lo > 4 ? [1] : [1, 2, 5];
  const ticks: number[] = [];
  for (let e = lo; e <= hi; e++) {
    for (const s of steps) {
      const v = s * 10 ** e;
      if (v <= 10 ** hi) ticks.push(v);
    }
  }
  return { domain: [10 ** lo, 10 ** hi], ticks };
}

/** `count` evenly spaced time ticks across [min, max] (a single tick when equal). */
export function timeTicks(min: number, max: number, count = 6): number[] {
  if (max <= min) return [min];
  return Array.from({ length: count }, (_, i) => Math.round(min + ((max - min) * i) / (count - 1)));
}
