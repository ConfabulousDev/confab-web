import { useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { TrendsCard } from './TrendsCard';
import { DollarIcon } from '@/components/icons';
import { formatCostCompact, formatTokenCount } from '@/utils/tokenStats';
import type {
  TrendsCostDistributionCard as TrendsCostDistributionCardData,
  TrendsCostDistributionBucket,
} from '@/schemas/api';
import styles from './TrendsCostDistributionCard.module.css';

// Which value the bars (and the tooltip's lead line) encode: per-band session
// count, or per-band total $. Defaults to 'count' so the card is unchanged on load.
type BarMetric = 'count' | 'cost';

interface TrendsCostDistributionCardProps {
  data: TrendsCostDistributionCardData | null;
  /**
   * When a model filter is active the histogram unit becomes per-(session,
   * model): each bar counts (session, selected-model) pairs and only the selected
   * model's cost in a session contributes. Drives the ⓘ caveat + the chart
   * label + tooltip wording.
   */
  modelFilterActive?: boolean;
}

const FILTER_CAVEAT =
  'A model filter is active: bars count (session, model) pairs and reflect only the selected model’s cost in each session, not full-session cost.';

// One bar per cost band: height encodes the session (or session-model-pair)
// count; the band total is surfaced on hover via CostDistributionTooltip.
// Recharts nests the bar's bucket row under payload[0].payload; the tooltip
// only reads these three fields (the bucket also carries lo/hi).
interface TooltipPayloadEntry {
  value: number;
  payload: {
    label: string;
    session_count: number;
    total_usd: string;
  };
}

interface CostDistributionTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadEntry[];
  /** "sessions" or "session-model pairs" — matches the chart label / a11y wording. */
  unit: string;
  /** Active bar metric; the matching line leads the tooltip. Defaults to 'count'. */
  metric?: BarMetric;
}

// Hover card for a single band: label, count + unit, and the band's total $.
// Whichever metric the bars encode leads (the other line still shows, just second).
// Exported for unit testing (Recharts renders tooltips only on hover, which
// jsdom can't lay out).
export function CostDistributionTooltip({
  active,
  payload,
  unit,
  metric = 'count',
}: CostDistributionTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const first = payload[0];
  if (!first) return null;
  const row = first.payload;

  const countLine = (
    <div className={styles.tooltipValue}>
      {row.session_count} {unit}
    </div>
  );
  const totalLine = (
    <div className={styles.tooltipTotal}>
      {formatCostCompact(parseFloat(row.total_usd))} total
    </div>
  );

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{row.label}</div>
      {metric === 'cost' ? (
        <>
          {totalLine}
          {countLine}
        </>
      ) : (
        <>
          {countLine}
          {totalLine}
        </>
      )}
    </div>
  );
}

// Stat tile for a single summary stat (cost-green via theme tokens, light + dark).
function StatTile({ label, usd }: { label: string; usd: string }) {
  return (
    <div className={styles.tile}>
      <span className={styles.tileLabel}>{label}</span>
      <span className={styles.tileValue}>{formatCostCompact(parseFloat(usd))}</span>
    </div>
  );
}

// Two-button in-card toggle flipping bar height between session count and total $.
// Button text is stable (no sessions-vs-pairs swap under a model filter) — that
// nuance stays in the chart label + ⓘ caveat. Mirrors TrendsTopSessionsCard's selector.
const METRIC_OPTIONS: { value: BarMetric; label: string }[] = [
  { value: 'count', label: 'Sessions' },
  { value: 'cost', label: 'Total $' },
];

function MetricToggle({
  metric,
  onMetricChange,
}: {
  metric: BarMetric;
  onMetricChange: (m: BarMetric) => void;
}) {
  return (
    <span className={styles.metricToggle} role="group" aria-label="Bar metric">
      {METRIC_OPTIONS.map(({ value, label }) => (
        <button
          key={value}
          type="button"
          className={styles.metricOption}
          aria-pressed={value === metric}
          onClick={() => onMetricChange(value)}
        >
          {label}
        </button>
      ))}
    </span>
  );
}

export function TrendsCostDistributionCard({ data, modelFilterActive }: TrendsCostDistributionCardProps) {
  // Local-only toggle; defaults to 'count' so first render is unchanged. Resets on
  // remount (e.g. a filter change), which is acceptable — no URL/storage plumbing.
  const [metric, setMetric] = useState<BarMetric>('count');

  if (!data) return null;

  // A timeout degrades to an empty card; surface a distinct notice (not the
  // "no data" empty state) so the user knows to narrow scope.
  if (data.timed_out) {
    return (
      <TrendsCard title="Cost Distribution" icon={DollarIcon}>
        <p className={styles.notice}>
          The cost distribution timed out for this range. Try narrowing the date range or removing
          some filters.
        </p>
      </TrendsCard>
    );
  }

  // No sessions carry per-session cost data → render nothing (mirrors Cost by Model).
  if (data.covered_session_count === 0) return null;

  const unit = modelFilterActive ? 'session-model pairs' : 'sessions';
  // In cost mode the bars encode total $; in count mode they encode the data-point
  // count (sessions, or session-model pairs under a filter).
  let chartLabel: string;
  if (metric === 'cost') {
    chartLabel = 'Total cost per cost band';
  } else if (modelFilterActive) {
    chartLabel = 'Session-model pairs per cost band';
  } else {
    chartLabel = 'Sessions per cost band';
  }

  // Recharts needs a numeric dataKey; total_usd is a decimal string, so cost mode
  // reads it through an accessor (leaves the bucket rows — and the tooltip payload —
  // untouched).
  const barDataKey =
    metric === 'cost'
      ? (b: TrendsCostDistributionBucket) => parseFloat(b.total_usd)
      : 'session_count';

  // Cost mode fills the bars with the canonical money green (matches CostAmount);
  // count mode keeps the neutral accent.
  const barFill = metric === 'cost' ? 'var(--color-cost)' : 'var(--color-accent)';

  // Always-visible Y-axis labels so magnitude is readable in a static screenshot
  // (no hover). Metric-aware: cost mode abbreviates dollars ($1.2K/$1.2M), count
  // mode abbreviates plain numbers (1.2k/1.2M) — both via the shared formatters.
  const yTickFormatter = (value: number) =>
    metric === 'cost' ? formatCostCompact(value) : formatTokenCount(value);

  return (
    <div className={styles.wrapper}>
      <TrendsCard
        title="Cost Distribution"
        icon={DollarIcon}
        caveat={modelFilterActive ? FILTER_CAVEAT : undefined}
      >
        {data.stats && (
          <div className={styles.tiles}>
            <StatTile label="avg" usd={data.stats.avg} />
            <StatTile label="p50" usd={data.stats.p50} />
            <StatTile label="p90" usd={data.stats.p90} />
            <StatTile label="p99" usd={data.stats.p99} />
          </div>
        )}

        <div className={styles.chartContainer}>
          <div className={styles.chartHeader}>
            <div className={styles.chartLabel}>{chartLabel}</div>
            <MetricToggle metric={metric} onMetricChange={setMetric} />
          </div>
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={data.buckets} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
              {/* Faint horizontal gridlines so the rough level reads across the full
                  chart width — kept low-opacity so it never competes with the bars. */}
              <CartesianGrid
                horizontal
                vertical={false}
                stroke="var(--color-text-muted)"
                strokeDasharray="3 3"
                strokeOpacity={0.25}
              />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
                axisLine={false}
                tickLine={false}
                angle={-45}
                textAnchor="end"
                tickMargin={8}
                height={72}
                interval={0}
              />
              {/* Subtle always-visible axis: muted small ticks, no axis/tick lines,
                  metric-aware numeric labels so screenshots convey magnitude. */}
              <YAxis
                domain={[0, 'dataMax']}
                tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
                axisLine={false}
                tickLine={false}
                tickFormatter={yTickFormatter}
                width={40}
                tickCount={4}
              />
              <Tooltip
                content={<CostDistributionTooltip unit={unit} metric={metric} />}
                cursor={{ fill: 'var(--color-bg-primary)' }}
              />
              <Bar
                dataKey={barDataKey}
                fill={barFill}
                radius={[3, 3, 0, 0]}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Coverage caption — NOT a reconciliation line. Sub-cent sessions are
            excluded, so the covered count is sessions priced ≥ $0.01; percentiles
            reflect that priced subset (also partial vs v2 during backfill). */}
        <p className={styles.caption}>
          Covers {data.covered_session_count} of {data.total_session_count} sessions priced ≥ $0.01;
          percentiles reflect this subset.
        </p>
      </TrendsCard>
    </div>
  );
}
