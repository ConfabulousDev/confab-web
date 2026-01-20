import { TrendsCard } from './TrendsCard';
import { WrenchIcon } from '@/components/icons';
import type { TrendsToolsCard as TrendsToolsCardData } from '@/schemas/api';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsToolsCard.module.css';

interface TrendsToolsCardProps {
  data: TrendsToolsCardData | null;
}

interface ToolChartData {
  name: string;
  displayName: string;
  success: number;
  errors: number;
  total: number;
}

/**
 * Format tool name for display. MCP tools like "mcp__linear-server__create_issue"
 * become just "create_issue". Built-in tools are unchanged.
 */
function formatToolName(name: string): string {
  const mcpMatch = name.match(/^mcp__[^_]+(?:-[^_]+)*__(.+)$/);
  if (mcpMatch?.[1]) {
    return mcpMatch[1];
  }
  return name;
}

function prepareChartData(toolStats: Record<string, { success: number; errors: number }>): ToolChartData[] {
  return Object.entries(toolStats)
    .map(([name, stats]) => ({
      name,
      displayName: formatToolName(name),
      success: stats.success,
      errors: stats.errors,
      total: stats.success + stats.errors,
    }))
    .sort((a, b) => b.total - a.total)
    .slice(0, 15); // Show top 15 tools
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    dataKey: string;
    color: string;
    payload: ToolChartData;
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const firstPayload = payload[0];
  if (!firstPayload) return null;
  const toolName = firstPayload.payload.name;
  const success = payload.find((p) => p.dataKey === 'success')?.value ?? 0;
  const errors = payload.find((p) => p.dataKey === 'errors')?.value ?? 0;
  const total = success + errors;

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{toolName}</div>
      <div className={styles.tooltipRow}>
        <span className={styles.tooltipDot} style={{ backgroundColor: '#f97316' }} />
        <span>Success: {success.toLocaleString()}</span>
      </div>
      {errors > 0 && (
        <div className={styles.tooltipRow}>
          <span className={styles.tooltipDot} style={{ backgroundColor: 'var(--color-error)' }} />
          <span>Errors: {errors.toLocaleString()}</span>
        </div>
      )}
      <div className={styles.tooltipTotal}>Total: {total.toLocaleString()}</div>
    </div>
  );
}

export function TrendsToolsCard({ data }: TrendsToolsCardProps) {
  if (!data || data.total_calls === 0) return null;

  const chartData = prepareChartData(data.tool_stats);

  // Calculate dynamic height based on number of tools
  const chartHeight = Math.max(120, chartData.length * 28);

  // Calculate dynamic YAxis width based on longest display label
  const maxLabelLength = Math.max(...chartData.map((d) => d.displayName.length));
  const yAxisWidth = Math.max(40, maxLabelLength * 7 + 8);

  const subtitle =
    data.total_errors > 0
      ? `${data.total_calls.toLocaleString()} calls (${data.total_errors.toLocaleString()} errors)`
      : `${data.total_calls.toLocaleString()} calls`;

  return (
    <TrendsCard title="Tools" icon={WrenchIcon} subtitle={subtitle}>
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
              tickFormatter={(value) => (value === 0 ? '' : value.toLocaleString())}
            />
            <YAxis
              type="category"
              dataKey="displayName"
              axisLine={false}
              tickLine={false}
              tick={{ fontSize: 11, fill: 'var(--color-text-secondary)' }}
              width={yAxisWidth}
            />
            <Tooltip
              content={<CustomTooltip />}
              cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
              wrapperStyle={{ transition: 'none' }}
              isAnimationActive={false}
            />
            <Bar
              dataKey="success"
              stackId="stack"
              fill="#f97316"
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
    </TrendsCard>
  );
}
