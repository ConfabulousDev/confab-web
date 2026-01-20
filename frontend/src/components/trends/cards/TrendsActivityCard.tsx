import { TrendsCard, StatRow } from './TrendsCard';
import { CodeIcon, FileIcon, PlusIcon, MinusIcon } from '@/components/icons';
import type { TrendsActivityCard as TrendsActivityCardData } from '@/schemas/api';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsActivityCard.module.css';

interface TrendsActivityCardProps {
  data: TrendsActivityCardData | null;
}

function formatNumber(n: number): string {
  if (n >= 1_000_000_000) {
    return `${(n / 1_000_000_000).toFixed(1)}B`;
  }
  if (n >= 1_000_000) {
    return `${(n / 1_000_000).toFixed(1)}M`;
  }
  if (n >= 1_000) {
    return `${(n / 1_000).toFixed(1)}K`;
  }
  return n.toLocaleString();
}

// Format date for chart axis
function formatChartDate(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    value: number;
    payload: { date: string; session_count: number };
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const firstPayload = payload[0];
  if (!firstPayload) return null;
  const item = firstPayload.payload;
  const date = new Date(item.date + 'T00:00:00');
  const formattedDate = date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipDate}>{formattedDate}</div>
      <div className={styles.tooltipValue}>
        {item.session_count} session{item.session_count !== 1 ? 's' : ''}
      </div>
    </div>
  );
}

export function TrendsActivityCard({ data }: TrendsActivityCardProps) {
  if (!data) return null;

  const hasChartData = data.daily_session_counts.length > 1;

  return (
    <TrendsCard
      title="Code Activity"
      icon={CodeIcon}
    >
      <StatRow
        label="Files Read"
        value={formatNumber(data.total_files_read)}
        icon={FileIcon}
      />
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
            <BarChart data={data.daily_session_counts} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
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
                content={<CustomTooltip />}
                cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
              />
              <Bar
                dataKey="session_count"
                fill="var(--color-accent)"
                radius={[2, 2, 0, 0]}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </TrendsCard>
  );
}
