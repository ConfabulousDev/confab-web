import { TrendsCard, StatRow } from './TrendsCard';
import { TokenIcon } from '@/components/icons';
import type { TrendsTokensCard as TrendsTokensCardData } from '@/schemas/api';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsTokensCard.module.css';

interface TrendsTokensCardProps {
  data: TrendsTokensCardData | null;
}

function formatTokens(n: number): string {
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

function formatCost(usd: string): string {
  const value = parseFloat(usd);
  if (value >= 1) {
    return `$${value.toFixed(2)}`;
  }
  return `$${value.toFixed(4)}`;
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
    payload: { date: string; cost_usd: string };
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
      <div className={styles.tooltipValue}>{formatCost(item.cost_usd)}</div>
    </div>
  );
}

export function TrendsTokensCard({ data }: TrendsTokensCardProps) {
  if (!data) return null;

  const totalTokens = data.total_input_tokens + data.total_output_tokens;

  // Prepare chart data
  const chartData = data.daily_costs.map((d) => ({
    ...d,
    costValue: parseFloat(d.cost_usd),
  }));

  const hasChartData = chartData.length > 1;

  return (
    <TrendsCard
      title="Tokens & Cost"
      icon={TokenIcon}
    >
      <StatRow
        label="Total Cost"
        value={<span style={{ color: '#22c55e', fontWeight: 600 }}>{formatCost(data.total_cost_usd)}</span>}
      />
      <StatRow
        label="Total Tokens"
        value={formatTokens(totalTokens)}
      />
      <StatRow
        label="Input / Output"
        value={`${formatTokens(data.total_input_tokens)} / ${formatTokens(data.total_output_tokens)}`}
      />
      <StatRow
        label="Cache (Create / Read)"
        value={`${formatTokens(data.total_cache_creation_tokens)} / ${formatTokens(data.total_cache_read_tokens)}`}
      />

      {hasChartData && (
        <div className={styles.chartContainer}>
          <div className={styles.chartLabel}>Daily Cost</div>
          <ResponsiveContainer width="100%" height={140}>
            <AreaChart data={chartData} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
              <defs>
                <linearGradient id="costGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
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
              <YAxis hide domain={[0, 'dataMax']} />
              <Tooltip
                content={<CustomTooltip />}
                cursor={{ stroke: 'var(--color-border)', strokeDasharray: '3 3' }}
              />
              <Area
                type="monotone"
                dataKey="costValue"
                stroke="#22c55e"
                strokeWidth={2}
                fill="url(#costGradient)"
                dot={{ r: 3, fill: '#22c55e', strokeWidth: 0 }}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </TrendsCard>
  );
}
