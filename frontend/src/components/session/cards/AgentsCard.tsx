import { CardWrapper, CardLoading } from './Card';
import { UsersIcon } from '@/components/icons';
import type { AgentsCardData } from '@/schemas/api';
import type { CardProps } from './types';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './AgentsCard.module.css';

interface AgentChartData {
  name: string;
  success: number;
  errors: number;
  total: number;
}

function prepareChartData(
  agentStats: Record<string, { success: number; errors: number }>
): AgentChartData[] {
  return Object.entries(agentStats)
    .map(([name, stats]) => ({
      name,
      success: stats.success,
      errors: stats.errors,
      total: stats.success + stats.errors,
    }))
    .sort((a, b) => b.total - a.total); // Longest bar first
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    dataKey: string;
    color: string;
    payload: AgentChartData;
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const firstPayload = payload[0];
  if (!firstPayload) return null;
  const agentName = firstPayload.payload.name;
  const success = payload.find((p) => p.dataKey === 'success')?.value ?? 0;
  const errors = payload.find((p) => p.dataKey === 'errors')?.value ?? 0;
  const total = success + errors;

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{agentName}</div>
      <div className={styles.tooltipRow}>
        <span
          className={styles.tooltipDot}
          style={{ backgroundColor: 'var(--color-success)' }}
        />
        <span>Success: {success}</span>
      </div>
      {errors > 0 && (
        <div className={styles.tooltipRow}>
          <span
            className={styles.tooltipDot}
            style={{ backgroundColor: 'var(--color-error)' }}
          />
          <span>Errors: {errors}</span>
        </div>
      )}
      <div className={styles.tooltipTotal}>Total: {total}</div>
    </div>
  );
}

export function AgentsCard({ data, loading }: CardProps<AgentsCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Agents" icon={UsersIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  // Don't render the card if no agents were used
  if (data.total_invocations === 0) return null;

  const chartData = prepareChartData(data.agent_stats);

  // Safety check: don't render if no chart data (shouldn't happen if total_invocations > 0)
  if (chartData.length === 0) return null;

  // Calculate dynamic height based on number of agents (min 120px, 28px per agent)
  const chartHeight = Math.max(120, chartData.length * 28);

  // Calculate dynamic YAxis width based on longest label (~7px per char at 11px font)
  const maxLabelLength = Math.max(...chartData.map((d) => d.name.length));
  const yAxisWidth = Math.max(40, maxLabelLength * 7 + 8);

  const errorCount = chartData.reduce((sum, d) => sum + d.errors, 0);
  const subtitle =
    errorCount > 0
      ? `${data.total_invocations} invocations (${errorCount} error${errorCount !== 1 ? 's' : ''})`
      : `${data.total_invocations} invocations`;

  return (
    <CardWrapper title="Agents" icon={UsersIcon} subtitle={subtitle}>
      <div className={styles.chartContainer} style={{ height: chartHeight }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={chartData}
            layout="vertical"
            margin={{ top: 0, right: 24, left: 0, bottom: 0 }}
            barSize={16}
          >
            <XAxis
              type="number"
              axisLine={false}
              tickLine={false}
              tick={{ fontSize: 10, fill: 'var(--color-text-tertiary)' }}
              tickFormatter={(value) => (value === 0 ? '' : String(value))}
            />
            <YAxis
              type="category"
              dataKey="name"
              axisLine={false}
              tickLine={false}
              tick={{ fontSize: 11, fill: 'var(--color-text-secondary)' }}
              width={yAxisWidth}
            />
            <Tooltip
              content={<CustomTooltip />}
              cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
            />
            <Bar
              dataKey="success"
              stackId="stack"
              fill="var(--color-success)"
              radius={[2, 2, 2, 2]}
              isAnimationActive={false}
            />
            <Bar
              dataKey="errors"
              stackId="stack"
              fill="var(--color-error)"
              radius={[2, 2, 2, 2]}
              isAnimationActive={false}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </CardWrapper>
  );
}
