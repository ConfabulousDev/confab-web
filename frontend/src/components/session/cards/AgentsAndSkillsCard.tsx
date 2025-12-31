import { CardWrapper, StatRow, CardLoading } from './Card';
import { UsersIcon, ZapIcon } from '@/components/icons';
import type { AgentsAndSkillsCardData } from '@/schemas/api';
import type { CardProps } from './types';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import styles from './AgentsAndSkillsCard.module.css';

// Color schemes for agents vs skills
const COLORS = {
  agent: {
    success: '#3B82F6', // Blue
    error: '#EF4444', // Red
  },
  skill: {
    success: '#8B5CF6', // Purple
    error: '#EF4444', // Red
  },
};

interface ChartDataItem {
  name: string;
  success: number;
  errors: number;
  total: number;
  type: 'agent' | 'skill';
}

function prepareChartData(
  agentStats: Record<string, { success: number; errors: number }>,
  skillStats: Record<string, { success: number; errors: number }>
): ChartDataItem[] {
  const agentData: ChartDataItem[] = Object.entries(agentStats).map(([name, stats]) => ({
    name,
    success: stats.success,
    errors: stats.errors,
    total: stats.success + stats.errors,
    type: 'agent' as const,
  }));

  const skillData: ChartDataItem[] = Object.entries(skillStats).map(([name, stats]) => ({
    name,
    success: stats.success,
    errors: stats.errors,
    total: stats.success + stats.errors,
    type: 'skill' as const,
  }));

  // Combine and sort by total (longest bar first)
  return [...agentData, ...skillData].sort((a, b) => b.total - a.total);
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    dataKey: string;
    color: string;
    payload: ChartDataItem;
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const firstPayload = payload[0];
  if (!firstPayload) return null;
  const item = firstPayload.payload;
  const success = payload.find((p) => p.dataKey === 'success')?.value ?? 0;
  const errors = payload.find((p) => p.dataKey === 'errors')?.value ?? 0;
  const total = success + errors;
  const typeLabel = item.type === 'agent' ? 'Agent' : 'Skill';
  const colors = COLORS[item.type];

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>
        {item.name}
        <span className={styles.tooltipType}>{typeLabel}</span>
      </div>
      <div className={styles.tooltipRow}>
        <span className={styles.tooltipDot} style={{ backgroundColor: colors.success }} />
        <span>Success: {success}</span>
      </div>
      {errors > 0 && (
        <div className={styles.tooltipRow}>
          <span className={styles.tooltipDot} style={{ backgroundColor: colors.error }} />
          <span>Errors: {errors}</span>
        </div>
      )}
      <div className={styles.tooltipTotal}>Total: {total}</div>
    </div>
  );
}

export function AgentsAndSkillsCard({ data, loading }: CardProps<AgentsAndSkillsCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Agents and Skills" icon={UsersIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  const totalInvocations = data.agent_invocations + data.skill_invocations;

  // Don't render the card if nothing was used
  if (totalInvocations === 0) return null;

  const chartData = prepareChartData(data.agent_stats, data.skill_stats);

  // Calculate dynamic height based on number of items (28px per item, min 56px)
  const chartHeight = Math.max(56, chartData.length * 28);

  // Calculate dynamic YAxis width based on longest label (~7px per char at 11px font)
  const maxLabelLength = Math.max(...chartData.map((d) => d.name.length), 6);
  const yAxisWidth = Math.max(40, maxLabelLength * 7 + 8);

  // Find max value for integer ticks
  const maxTotal = Math.max(...chartData.map((d) => d.total));
  const tickCount = Math.min(maxTotal + 1, 6); // Max 6 ticks

  return (
    <CardWrapper title="Agents and Skills" icon={UsersIcon}>
      <div className={styles.stats}>
        <StatRow label="Agent invocations" value={data.agent_invocations} icon={UsersIcon} />
        <StatRow label="Skill invocations" value={data.skill_invocations} icon={ZapIcon} />
      </div>

      {chartData.length > 0 && (
        <>
          <div className={styles.legend}>
            <div className={styles.legendItem}>
              <span className={styles.legendDot} style={{ backgroundColor: COLORS.agent.success }} />
              <span>Agents</span>
            </div>
            <div className={styles.legendItem}>
              <span className={styles.legendDot} style={{ backgroundColor: COLORS.skill.success }} />
              <span>Skills</span>
            </div>
          </div>

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
                  tick={maxTotal > 2 ? { fontSize: 10, fill: 'var(--color-text-tertiary)' } : false}
                  tickCount={tickCount}
                  allowDecimals={false}
                  tickFormatter={(value) => (value === 0 ? '' : String(Math.floor(value)))}
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
                  wrapperStyle={{ zIndex: 9999, pointerEvents: 'none', transition: 'none' }}
                  allowEscapeViewBox={{ x: true, y: true }}
                  isAnimationActive={false}
                />
                <Bar dataKey="success" stackId="stack" radius={[2, 2, 2, 2]} isAnimationActive={false}>
                  {chartData.map((entry, index) => (
                    <Cell key={`success-${index}`} fill={COLORS[entry.type].success} />
                  ))}
                </Bar>
                <Bar dataKey="errors" stackId="stack" radius={[2, 2, 2, 2]} isAnimationActive={false}>
                  {chartData.map((entry, index) => (
                    <Cell key={`error-${index}`} fill={COLORS[entry.type].error} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </>
      )}
    </CardWrapper>
  );
}
