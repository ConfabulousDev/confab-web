import { useMemo } from 'react';
import { TrendsCard, StatRow } from './TrendsCard';
import { CodeIcon, FileIcon, PlusIcon, MinusIcon } from '@/components/icons';
import type { TrendsActivityCard as TrendsActivityCardData } from '@/schemas/api';
import {
  providerLabel,
  getProviderMetadataOrFallback,
} from '@/utils/providers';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsActivityCard.module.css';

const UNKNOWN_PROVIDER_COLOR = '#9ca3af';
// Synthetic single-stack key used when no per_provider breakdown is on the
// wire (older backends emit `{}` for every day). Cannot collide with a
// canonical provider id.
const FALLBACK_STACK_KEY = '__total__';

const FILES_READ_CAVEAT = 'Excludes Codex sessions (no Read tool)';

interface TrendsActivityCardProps {
  data: TrendsActivityCardData | null;
  // Canonical provider ids in the filtered window (TrendsResponse.providers_present).
  // Drives the Files Read row's three-state behavior: hidden when only Codex
  // (no Read tool), caveat tooltip when mixed Claude+Codex, unchanged otherwise.
  providersPresent: string[];
}

function formatNumber(n: number): string {
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

function formatChartDate(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function providerColor(providerId: string): string {
  if (providerId === FALLBACK_STACK_KEY) return 'var(--color-accent)';
  const meta = getProviderMetadataOrFallback(providerId, 'neutral');
  return meta?.brandColor ?? UNKNOWN_PROVIDER_COLOR;
}

interface ChartRow {
  date: string;
  total: number;
  [providerId: string]: string | number;
}

interface TooltipPayloadEntry {
  name: string;
  value: number;
  color: string;
  payload: ChartRow;
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadEntry[];
  showBreakdown: boolean;
}

function CustomTooltip({ active, payload, showBreakdown }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const row = payload[0]!.payload;
  const formattedDate = new Date(row.date + 'T00:00:00').toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });
  const nonZero = payload.filter((p) => p.value > 0);

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipDate}>{formattedDate}</div>
      <div className={styles.tooltipValue}>
        {row.total} session{row.total !== 1 ? 's' : ''}
      </div>
      {showBreakdown && nonZero.length > 0 && (
        <div className={styles.tooltipBreakdown}>
          {nonZero.map((p) => (
            <div key={p.name} className={styles.tooltipRow}>
              <span className={styles.tooltipDot} style={{ background: p.color }} />
              <span className={styles.tooltipProviderLabel}>{providerLabel(p.name)}</span>
              <span className={styles.tooltipProviderValue}>{p.value}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function TrendsActivityCard({ data, providersPresent }: TrendsActivityCardProps) {
  // Stacked-bar series: union of canonical providers across days, alphabetical.
  // Falls back to a single synthetic stack when no per_provider data is on the
  // wire (older backends).
  const stackProviderIds: string[] = useMemo(() => {
    if (!data) return [];
    const seen = new Set<string>();
    for (const d of data.daily_session_counts) {
      for (const id of Object.keys(d.per_provider)) {
        if (d.per_provider[id] !== 0) seen.add(id);
      }
    }
    if (seen.size > 0) return Array.from(seen).sort();
    return data.daily_session_counts.length > 0 ? [FALLBACK_STACK_KEY] : [];
  }, [data]);

  const chartData: ChartRow[] = useMemo(() => {
    if (!data) return [];
    return data.daily_session_counts.map((d) => {
      const row: ChartRow = { date: d.date, total: d.session_count };
      for (const providerId of stackProviderIds) {
        row[providerId] =
          providerId === FALLBACK_STACK_KEY
            ? d.session_count
            : d.per_provider[providerId] ?? 0;
      }
      return row;
    });
  }, [data, stackProviderIds]);

  if (!data) return null;

  // Files Read three-state: hide when only Codex (always 0 by design, mirrors
  // CF-439); caveat when mixed Claude+Codex; unchanged otherwise.
  const hasCodex = providersPresent.includes('codex');
  const onlyCodex = hasCodex && providersPresent.length === 1;

  const hasChartData = chartData.length > 1;
  // Fallback path always emits exactly one stack key, so length > 1 implies
  // real per-provider stacking and the breakdown is meaningful.
  const tooltipShowBreakdown = stackProviderIds.length > 1;

  return (
    <TrendsCard title="Code Activity" icon={CodeIcon}>
      {!onlyCodex && (
        <StatRow
          label="Files Read"
          value={
            <>
              {formatNumber(data.total_files_read)}
              {hasCodex && (
                <span
                  className={styles.caveatIcon}
                  title={FILES_READ_CAVEAT}
                  aria-label={FILES_READ_CAVEAT}
                >
                  ⓘ
                </span>
              )}
            </>
          }
          icon={FileIcon}
        />
      )}
      <StatRow
        label="Files Modified"
        value={formatNumber(data.total_files_modified)}
        icon={FileIcon}
      />
      <StatRow
        label="Lines Added"
        value={`+${formatNumber(data.total_lines_added)}`}
        icon={PlusIcon}
      />
      <StatRow
        label="Lines Removed"
        value={`-${formatNumber(data.total_lines_removed)}`}
        icon={MinusIcon}
      />

      {hasChartData && (
        <div className={styles.chartContainer}>
          <div className={styles.chartLabel}>Sessions per Day</div>
          <ResponsiveContainer width="100%" height={140}>
            <BarChart data={chartData} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
              <XAxis
                dataKey="date"
                tickFormatter={formatChartDate}
                tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
                axisLine={false}
                tickLine={false}
                angle={-45}
                textAnchor="end"
                height={40}
              />
              <YAxis hide domain={[0, 'dataMax']} />
              <Tooltip
                content={<CustomTooltip showBreakdown={tooltipShowBreakdown} />}
                cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
              />
              {stackProviderIds.map((providerId) => (
                <Bar
                  key={providerId}
                  dataKey={providerId}
                  stackId="sessions"
                  fill={providerColor(providerId)}
                  isAnimationActive={false}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </TrendsCard>
  );
}
