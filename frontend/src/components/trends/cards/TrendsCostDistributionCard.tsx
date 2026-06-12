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
   * model's cost in a session contributes. Drives the ⓘ caveat + a11y wording.
   */
  modelFilterActive?: boolean;
}

const FILTER_CAVEAT =
  'A model filter is active: bars count (session, model) pairs and reflect only the selected model’s cost in each session, not full-session cost.';

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
  const maxCount = data.buckets.reduce((m, b) => Math.max(m, b.session_count), 0) || 1;

  return (
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

      <div
        className={styles.chart}
        role="list"
        aria-label={modelFilterActive ? 'Per-(session, model) cost distribution' : 'Per-session cost distribution'}
      >
        {data.buckets.map((b) => (
          <div
            key={b.label}
            className={styles.col}
            role="listitem"
            aria-label={`${b.label}: ${b.session_count} ${unit}, ${formatCostCompact(parseFloat(b.total_usd))} total`}
          >
            {/* Total $ is always visible (a11y — not hover-gated). */}
            <span className={styles.barTotal}>{formatCostCompact(parseFloat(b.total_usd))}</span>
            <div className={styles.barTrack}>
              <div
                className={styles.bar}
                style={{ height: `${(b.session_count / maxCount) * 100}%` }}
                aria-hidden="true"
              />
            </div>
            <span className={styles.barAxis}>{b.label}</span>
          </div>
        ))}
      </div>

      {/* Coverage caption — NOT a reconciliation line. Percentiles reflect the
          partial v2 subset during backfill. */}
      <p className={styles.caption}>
        Covers {data.covered_session_count} of {data.total_session_count} sessions with cost data;
        percentiles reflect this subset.
      </p>
    </TrendsCard>
  );
}
