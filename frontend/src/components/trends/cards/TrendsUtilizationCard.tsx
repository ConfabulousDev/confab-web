import { TrendsCard } from './TrendsCard';
import { ZapIcon } from '@/components/icons';
import type { TrendsUtilizationCard as TrendsUtilizationCardData } from '@/schemas/api';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ReferenceLine,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsUtilizationCard.module.css';

interface TrendsUtilizationCardProps {
  data: TrendsUtilizationCardData | null;
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
    payload: { date: string; utilization_pct: number | null };
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

  const utilizationText = item.utilization_pct != null
    ? `${item.utilization_pct.toFixed(1)}%`
    : 'No data';

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipDate}>{formattedDate}</div>
      <div className={styles.tooltipValue}>{utilizationText}</div>
    </div>
  );
}

export function TrendsUtilizationCard({ data }: TrendsUtilizationCardProps) {
  if (!data) return null;

  // Prepare chart data - convert null to 0 for rendering
  const chartData = data.daily_utilization.map((d) => ({
    ...d,
    utilizationValue: d.utilization_pct ?? 0,
  }));

  const hasChartData = chartData.length > 1;

  return (
    <TrendsCard
      title="Assistant Utilization"
      icon={ZapIcon}
    >
      {hasChartData && (
        <div className={styles.chartContainer}>
          <div className={styles.chartLabel}>Daily Utilization</div>
          <ResponsiveContainer width="100%" height={160}>
            <AreaChart data={chartData} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
              <defs>
                <linearGradient id="utilizationGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                </linearGradient>
              </defs>
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
              <YAxis
                hide
                domain={[0, 100]}
              />
              <ReferenceLine
                y={50}
                stroke="var(--color-text-muted)"
                strokeDasharray="3 3"
                strokeOpacity={0.5}
                label={{
                  value: '50%',
                  position: 'right',
                  fontSize: 10,
                  fill: 'var(--color-text-muted)',
                }}
              />
              <Tooltip
                content={<CustomTooltip />}
                cursor={{ stroke: 'var(--color-border)', strokeDasharray: '3 3' }}
              />
              <Area
                type="monotone"
                dataKey="utilizationValue"
                stroke="#8b5cf6"
                strokeWidth={2}
                fill="url(#utilizationGradient)"
                dot={{ r: 3, fill: '#8b5cf6', strokeWidth: 0 }}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </TrendsCard>
  );
}
