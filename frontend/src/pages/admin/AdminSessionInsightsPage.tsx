import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  BarChart,
  Bar,
  ScatterChart,
  Scatter,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { adminAPI, APIError } from '@/services/api';
import Alert from '@/components/Alert';
import { PROVIDER_VALUES, providerChartColor, providerLabel } from '@/utils/providers';
import type { ProviderShareResponse, SessionPointsResponse } from '@/schemas/api';
import {
  RANGE_PRESETS,
  SIZE_METRICS,
  buildScatterSeries,
  buildShareRows,
  formatBucketLabel,
  insightsParams,
  logAxis,
  timeTicks,
  providerMarker,
  shareProviders,
  sizeMetric,
  type MarkerShape,
  type ScatterDatum,
  type SessionInsightsFilters,
  type ShareRow,
  type SizeMetric,
} from './sessionInsights';
import styles from './AdminSessionInsightsPage.module.css';

export interface InsightsUser {
  id: number;
  email: string;
}

export interface AdminSessionInsightsPageContentProps {
  filters: SessionInsightsFilters;
  onFiltersChange: (filters: SessionInsightsFilters) => void;
  users: InsightsUser[];
  share: ProviderShareResponse | null;
  points: SessionPointsResponse | null;
  isLoading: boolean;
  error: string | null;
  onPointClick: (sessionId: string) => void;
}

const AXIS_TICK = { fontSize: 11, fill: 'var(--color-text-muted)' };

/** Small inline glyph matching a scatter marker, for the legend. */
function MarkerGlyph({ shape, color }: { shape: MarkerShape; color: string }) {
  const common = { fill: color };
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" className={styles.glyph}>
      {shape === 'circle' && <circle cx="5" cy="5" r="4.5" {...common} />}
      {shape === 'square' && <rect x="1" y="1" width="8" height="8" {...common} />}
      {shape === 'triangle' && <polygon points="5,0.5 9.5,9.5 0.5,9.5" {...common} />}
      {shape === 'diamond' && <polygon points="5,0 10,5 5,10 0,5" {...common} />}
      {shape === 'cross' && <path d="M3.5 0h3v3.5H10v3H6.5V10h-3V6.5H0v-3h3.5z" {...common} />}
    </svg>
  );
}

function Legend({ items }: { items: { provider: string; count: number }[] }) {
  return (
    <ul className={styles.legend} aria-label="Providers">
      {items.map(({ provider, count }) => (
        <li key={provider} className={styles.legendItem}>
          <MarkerGlyph shape={providerMarker(provider)} color={providerChartColor(provider)} />
          <span>{providerLabel(provider)}</span>
          <span className={styles.legendCount}>{count.toLocaleString()}</span>
        </li>
      ))}
    </ul>
  );
}

interface ShareTooltipProps {
  active?: boolean;
  payload?: { dataKey?: string | number; value?: number; payload: ShareRow }[];
  bucketSize: ProviderShareResponse['bucket_size'];
  mode: 'count' | 'percent';
  counts: Map<number, ShareRow>;
}

function ShareTooltip({ active, payload, bucketSize, mode, counts }: ShareTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const row = payload[0]!.payload;
  const raw = counts.get(row.start);
  if (!raw) return null;
  const date = formatBucketLabel(row.start, bucketSize);
  const label = bucketSize === 'week' ? `Week of ${date}` : date;
  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{label}</div>
      <div className={styles.tooltipMeta}>
        {raw.total.toLocaleString()} session{raw.total === 1 ? '' : 's'}
      </div>
      {payload
        .filter((p) => (raw[String(p.dataKey)] ?? 0) > 0)
        .map((p) => {
          const provider = String(p.dataKey);
          const n = raw[provider] ?? 0;
          const pct = raw.total > 0 ? Math.round((n / raw.total) * 100) : 0;
          return (
            <div key={provider} className={styles.tooltipRow}>
              <span className={styles.tooltipDot} style={{ background: providerChartColor(provider) }} />
              <span>{providerLabel(provider)}</span>
              <span className={styles.tooltipValue}>
                {mode === 'percent' ? `${pct}% (${n.toLocaleString()})` : `${n.toLocaleString()} (${pct}%)`}
              </span>
            </div>
          );
        })}
    </div>
  );
}

interface PointTooltipProps {
  active?: boolean;
  payload?: { payload: ScatterDatum }[];
}

function PointTooltip({ active, payload }: PointTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const { point } = payload[0]!.payload;
  const repo = point.repo ? `${point.repo}${point.branch ? ` @ ${point.branch}` : ''}` : null;
  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{point.title || 'Untitled session'}</div>
      <div className={styles.tooltipMeta}>
        {providerLabel(point.provider)} · {point.owner_email}
      </div>
      {repo && <div className={styles.tooltipMeta}>{repo}</div>}
      <div className={styles.tooltipMeta}>{new Date(point.first_seen).toLocaleString()}</div>
      <dl className={styles.tooltipMetrics}>
        {SIZE_METRICS.map((m) => {
          const v = point[m.id];
          return (
            <div key={m.id} className={styles.tooltipRow}>
              <dt>{m.label}</dt>
              <dd className={styles.tooltipValue}>{v === null ? '—' : m.format(v)}</dd>
            </div>
          );
        })}
      </dl>
    </div>
  );
}

function ProviderShareCard({ share }: { share: ProviderShareResponse }) {
  const [mode, setMode] = useState<'count' | 'percent'>('count');
  const providers = useMemo(() => shareProviders(share), [share]);
  const rows = useMemo(() => buildShareRows(share, mode), [share, mode]);
  const counts = useMemo(() => new Map(buildShareRows(share, 'count').map((r) => [r.start, r])), [share]);
  const grandTotal = providers.reduce((sum, p) => sum + (share.totals[p] ?? 0), 0);

  return (
    <section className={styles.card} aria-labelledby="provider-share-heading">
      <div className={styles.cardHeader}>
        <h3 id="provider-share-heading">Provider share</h3>
        <div className={styles.segmented} role="group" aria-label="Value mode">
          {(['count', 'percent'] as const).map((m) => (
            <button
              key={m}
              type="button"
              className={styles.segment}
              aria-pressed={mode === m}
              onClick={() => setMode(m)}
            >
              {m === 'count' ? 'Count' : '%'}
            </button>
          ))}
        </div>
      </div>
      <p className={styles.description}>
        Sessions started per {share.bucket_size} (UTC), stacked by provider.
      </p>

      {grandTotal === 0 ? (
        <div className={styles.status}>No sessions in this range.</div>
      ) : (
        <>
          <Legend items={providers.map((p) => ({ provider: p, count: share.totals[p] ?? 0 }))} />
          <div className={styles.chart} data-testid="provider-share-chart">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={rows} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid vertical={false} stroke="var(--color-border-light)" />
                <XAxis
                  dataKey="start"
                  tickFormatter={(v: number) => formatBucketLabel(v, share.bucket_size)}
                  tick={AXIS_TICK}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={16}
                />
                <YAxis
                  tick={AXIS_TICK}
                  axisLine={false}
                  tickLine={false}
                  width={44}
                  allowDecimals={false}
                  domain={mode === 'percent' ? [0, 100] : [0, 'auto']}
                  tickFormatter={(v: number) => (mode === 'percent' ? `${v}%` : v.toLocaleString())}
                />
                <Tooltip
                  content={<ShareTooltip bucketSize={share.bucket_size} mode={mode} counts={counts} />}
                  cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
                />
                {providers.map((p) => (
                  <Bar
                    key={p}
                    dataKey={p}
                    stackId="share"
                    fill={providerChartColor(p)}
                    stroke="var(--color-bg-primary)"
                    strokeWidth={1}
                    isAnimationActive={false}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>

          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Provider</th>
                  <th>Sessions</th>
                  <th>Share</th>
                </tr>
              </thead>
              <tbody>
                {providers.map((p) => {
                  const n = share.totals[p] ?? 0;
                  return (
                    <tr key={p}>
                      <td>{providerLabel(p)}</td>
                      <td>{n.toLocaleString()}</td>
                      <td>{((n / grandTotal) * 100).toFixed(1)}%</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </section>
  );
}

function SizeDistributionCard({
  points,
  onPointClick,
}: {
  points: SessionPointsResponse;
  onPointClick: (sessionId: string) => void;
}) {
  const [metricId, setMetricId] = useState<SizeMetric>('lines');
  const metric = sizeMetric(metricId);
  const { series, excluded } = useMemo(() => buildScatterSeries(points.points, metricId), [points, metricId]);
  const axes = useMemo(() => {
    const data = series.flatMap((s) => s.data);
    const xs = data.map((d) => d.x);
    const minX = Math.min(...xs);
    const maxX = Math.max(...xs);
    const spanDays = (maxX - minX) / 86_400_000;
    const xDomain: [number, number] = [minX, maxX];
    return {
      y: logAxis(data.map((d) => d.y)),
      x: { domain: xDomain, ticks: timeTicks(minX, maxX) },
      xFormat: (v: number) =>
        new Date(v).toLocaleDateString(
          undefined,
          spanDays > 400 ? { month: 'short', year: 'numeric' } : { month: 'short', day: 'numeric' },
        ),
    };
  }, [series]);
  const capped = points.returned < points.total;

  return (
    <section className={styles.card} aria-labelledby="size-distribution-heading">
      <div className={styles.cardHeader}>
        <h3 id="size-distribution-heading">Session size distribution</h3>
        <label className={styles.inlineLabel}>
          Size
          <select
            className={styles.select}
            value={metricId}
            onChange={(e) => {
              const next = SIZE_METRICS.find((m) => m.id === e.target.value);
              if (next) setMetricId(next.id);
            }}
          >
            {SIZE_METRICS.map((m) => (
              <option key={m.id} value={m.id}>
                {m.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <p className={styles.description}>
        One point per session by start time, on a log scale. Click a point to open the session.
      </p>

      {capped && (
        <div className={styles.note} role="status">
          Showing {points.returned.toLocaleString()} of {points.total.toLocaleString()} sessions (a stable
          sample). Narrow the range or filters to see them all.
        </div>
      )}
      {excluded > 0 && (
        <div className={styles.note}>
          {excluded.toLocaleString()} session{excluded === 1 ? ' has' : 's have'} no {metric.noun}.
        </div>
      )}

      {series.length === 0 ? (
        <div className={styles.status}>
          {points.total === 0 ? 'No sessions in this range.' : `No sessions with ${metric.noun} in this range.`}
        </div>
      ) : (
        <>
          <Legend items={series.map((s) => ({ provider: s.provider, count: s.data.length }))} />
          <div className={styles.chart} data-testid="size-distribution-chart">
            <ResponsiveContainer width="100%" height={320}>
              <ScatterChart margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid stroke="var(--color-border-light)" />
                <XAxis
                  type="number"
                  dataKey="x"
                  domain={axes.x.domain}
                  ticks={axes.x.ticks}
                  tickFormatter={axes.xFormat}
                  padding={{ left: 12, right: 12 }}
                  tick={AXIS_TICK}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={24}
                />
                <YAxis
                  type="number"
                  dataKey="y"
                  scale="log"
                  domain={axes.y.domain}
                  ticks={axes.y.ticks}
                  tickFormatter={(v: number) => metric.format(v)}
                  tick={AXIS_TICK}
                  axisLine={false}
                  tickLine={false}
                  width={64}
                />
                <Tooltip content={<PointTooltip />} cursor={{ stroke: 'var(--color-border-dark)' }} />
                {series.map((s) => (
                  <Scatter
                    key={s.provider}
                    name={providerLabel(s.provider)}
                    data={s.data}
                    fill={providerChartColor(s.provider)}
                    fillOpacity={0.85}
                    stroke="var(--color-bg-primary)"
                    strokeWidth={1}
                    shape={providerMarker(s.provider)}
                    isAnimationActive={false}
                    cursor="pointer"
                    onClick={(entry: { payload?: ScatterDatum }) => {
                      if (entry.payload) onPointClick(entry.payload.point.id);
                    }}
                  />
                ))}
              </ScatterChart>
            </ResponsiveContainer>
          </div>
        </>
      )}
    </section>
  );
}

export function AdminSessionInsightsPageContent({
  filters,
  onFiltersChange,
  users,
  share,
  points,
  isLoading,
  error,
  onPointClick,
}: AdminSessionInsightsPageContentProps) {
  const toggleProvider = (p: string) => {
    const next = filters.providers.includes(p)
      ? filters.providers.filter((x) => x !== p)
      : [...filters.providers, p];
    onFiltersChange({ ...filters, providers: next });
  };

  return (
    <div>
      <div className={styles.card}>
        <h3>Session Insights</h3>
        <p className={styles.description}>
          Instance-wide metadata for every listable session, regardless of owner or sharing. Filters
          apply to both charts.
        </p>
        <div className={styles.filters}>
          <div className={styles.segmented} role="group" aria-label="Date range">
            {RANGE_PRESETS.map((r) => (
              <button
                key={r.id}
                type="button"
                className={styles.segment}
                aria-pressed={filters.range === r.id}
                onClick={() => onFiltersChange({ ...filters, range: r.id })}
              >
                {r.label}
              </button>
            ))}
          </div>
          <div className={styles.segmented} role="group" aria-label="Providers">
            {PROVIDER_VALUES.map((p) => (
              <button
                key={p}
                type="button"
                className={styles.segment}
                aria-pressed={filters.providers.includes(p)}
                onClick={() => toggleProvider(p)}
              >
                {providerLabel(p)}
              </button>
            ))}
          </div>
          <label className={styles.inlineLabel}>
            Owner
            <select
              className={styles.select}
              value={filters.userId ?? ''}
              onChange={(e) =>
                onFiltersChange({ ...filters, userId: e.target.value ? Number(e.target.value) : undefined })
              }
            >
              <option value="">All users</option>
              {users.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.email}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}
      {isLoading && <div className={styles.status}>Loading...</div>}
      {!isLoading && share && <ProviderShareCard share={share} />}
      {!isLoading && points && <SizeDistributionCard points={points} onPointClick={onPointClick} />}
    </div>
  );
}

function errorMessage(error: unknown): string | null {
  if (!error) return null;
  return error instanceof APIError ? error.message : 'Failed to load session insights.';
}

function AdminSessionInsightsPage() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<SessionInsightsFilters>({ range: '30d', providers: [] });
  const key = [filters.range, [...filters.providers].sort(), filters.userId ?? null];

  // `from` is computed at fetch time so the query key stays stable per preset.
  const shareQuery = useQuery({
    queryKey: ['admin', 'session-insights', 'provider-share', ...key],
    queryFn: () => adminAPI.getSessionInsightsProviderShare(insightsParams(filters)),
  });
  const pointsQuery = useQuery({
    queryKey: ['admin', 'session-insights', 'points', ...key],
    queryFn: () => adminAPI.getSessionInsightsPoints(insightsParams(filters)),
  });
  const usersQuery = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: () => adminAPI.listUsers(),
  });

  const users = useMemo(
    () =>
      (usersQuery.data?.users ?? [])
        .map((u) => ({ id: u.id, email: u.email }))
        .sort((a, b) => a.email.localeCompare(b.email)),
    [usersQuery.data],
  );

  return (
    <AdminSessionInsightsPageContent
      filters={filters}
      onFiltersChange={setFilters}
      users={users}
      share={shareQuery.data ?? null}
      points={pointsQuery.data ?? null}
      isLoading={shareQuery.isLoading || pointsQuery.isLoading}
      error={errorMessage(shareQuery.error ?? pointsQuery.error)}
      onPointClick={(id) => navigate(`/sessions/${id}`)}
    />
  );
}

export default AdminSessionInsightsPage;
