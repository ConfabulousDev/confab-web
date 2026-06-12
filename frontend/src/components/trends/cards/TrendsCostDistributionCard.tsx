import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { TrendsCard } from './TrendsCard';
import { DollarIcon } from '@/components/icons';
import { formatCostCompact } from '@/utils/tokenStats';
import type { TrendsCostDistributionCard as TrendsCostDistributionCardData } from '@/schemas/api';
import styles from './TrendsCostDistributionCard.module.css';

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
}

// Hover card for a single band: label, count + unit, and the band's total $.
// Exported for unit testing (Recharts renders tooltips only on hover, which
// jsdom can't lay out).
export function CostDistributionTooltip({ active, payload, unit }: CostDistributionTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const first = payload[0];
  if (!first) return null;
  const row = first.payload;

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{row.label}</div>
      <div className={styles.tooltipValue}>
        {row.session_count} {unit}
      </div>
      <div className={styles.tooltipTotal}>
        {formatCostCompact(parseFloat(row.total_usd))} total
      </div>
    </div>
  );
}

// Stat tile for a single percentile (cost-green via theme tokens, light + dark).
function PercentileTile({ label, usd }: { label: string; usd: string }) {
  return (
    <div className={styles.tile}>
      <span className={styles.tileLabel}>{label}</span>
      <span className={styles.tileValue}>{formatCostCompact(parseFloat(usd))}</span>
    </div>
  );
}

export function TrendsCostDistributionCard({ data, modelFilterActive }: TrendsCostDistributionCardProps) {
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
  const chartLabel = modelFilterActive
    ? 'Session-model pairs per cost band'
    : 'Sessions per cost band';

  return (
    <div className={styles.wrapper}>
      <TrendsCard
        title="Cost Distribution"
        icon={DollarIcon}
        caveat={modelFilterActive ? FILTER_CAVEAT : undefined}
      >
        {data.percentiles && (
          <div className={styles.tiles}>
            <PercentileTile label="p50" usd={data.percentiles.p50} />
            <PercentileTile label="p90" usd={data.percentiles.p90} />
            <PercentileTile label="p99" usd={data.percentiles.p99} />
          </div>
        )}

        <div className={styles.chartContainer}>
          <div className={styles.chartLabel}>{chartLabel}</div>
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={data.buckets} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
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
              <YAxis hide domain={[0, 'dataMax']} />
              <Tooltip
                content={<CostDistributionTooltip unit={unit} />}
                cursor={{ fill: 'var(--color-bg-primary)' }}
              />
              <Bar
                dataKey="session_count"
                fill="var(--color-accent)"
                radius={[3, 3, 0, 0]}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Coverage caption — NOT a reconciliation line. Percentiles reflect the
            partial v2 subset during backfill. */}
        <p className={styles.caption}>
          Covers {data.covered_session_count} of {data.total_session_count} sessions with cost data;
          percentiles reflect this subset.
        </p>
      </TrendsCard>
    </div>
  );
}
