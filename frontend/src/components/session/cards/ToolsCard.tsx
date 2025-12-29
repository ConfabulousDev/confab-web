import { CardWrapper, CardLoading } from './Card';
import type { ToolsCardData } from '@/schemas/api';
import type { CardProps } from './types';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './ToolsCard.module.css';

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
 * Full name is shown in tooltip on hover.
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
    .sort((a, b) => b.total - a.total); // Longest bar first
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

  // Get full tool name from the data entry (not the display name)
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
        <span className={styles.tooltipDot} style={{ backgroundColor: 'var(--color-success)' }} />
        <span>Success: {success}</span>
      </div>
      {errors > 0 && (
        <div className={styles.tooltipRow}>
          <span className={styles.tooltipDot} style={{ backgroundColor: 'var(--color-error)' }} />
          <span>Errors: {errors}</span>
        </div>
      )}
      <div className={styles.tooltipTotal}>Total: {total}</div>
    </div>
  );
}

export function ToolsCard({ data, loading }: CardProps<ToolsCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Tools">
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  // Don't render the card if no tools were used
  if (data.total_calls === 0) return null;

  const chartData = prepareChartData(data.tool_stats);

  // Calculate dynamic height based on number of tools (min 120px, 28px per tool)
  const chartHeight = Math.max(120, chartData.length * 28);

  // Calculate dynamic YAxis width based on longest display label (~7px per char at 11px font)
  const maxLabelLength = Math.max(...chartData.map((d) => d.displayName.length));
  const yAxisWidth = Math.max(40, maxLabelLength * 7 + 8);

  const subtitle =
    data.error_count > 0
      ? `${data.total_calls} calls (${data.error_count} error${data.error_count !== 1 ? 's' : ''})`
      : `${data.total_calls} calls`;

  return (
    <CardWrapper title="Tools" subtitle={subtitle}>
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
              dataKey="displayName"
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
            />
            <Bar
              dataKey="errors"
              stackId="stack"
              fill="var(--color-error)"
              radius={[2, 2, 2, 2]}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </CardWrapper>
  );
}
