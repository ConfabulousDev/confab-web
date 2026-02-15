import { TrendsCard } from './TrendsCard';
import { UsersIcon } from '@/components/icons';
import type { TrendsAgentsAndSkillsCard as TrendsAgentsAndSkillsCardData } from '@/schemas/api';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import styles from './TrendsAgentsAndSkillsCard.module.css';

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
  stats: Record<string, { success: number; errors: number }>,
  type: 'agent' | 'skill'
): ChartDataItem[] {
  return Object.entries(stats)
    .map(([name, s]) => ({
      name,
      success: s.success,
      errors: s.errors,
      total: s.success + s.errors,
      type,
    }))
    .sort((a, b) => b.total - a.total);
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
        <span>Success: {success.toLocaleString()}</span>
      </div>
      {errors > 0 && (
        <div className={styles.tooltipRow}>
          <span className={styles.tooltipDot} style={{ backgroundColor: colors.error }} />
          <span>Errors: {errors.toLocaleString()}</span>
        </div>
      )}
      <div className={styles.tooltipTotal}>Total: {total.toLocaleString()}</div>
    </div>
  );
}

interface SectionChartProps {
  data: ChartDataItem[];
}

function SectionChart({ data }: SectionChartProps) {
  const chartHeight = Math.max(80, data.length * 28);
  const maxLabelLength = Math.max(...data.map((d) => d.name.length), 4);
  const yAxisWidth = Math.max(40, maxLabelLength * 7 + 8);

  return (
    <div className={styles.chartContainer} style={{ height: chartHeight }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={data}
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
            dataKey="name"
            axisLine={false}
            tickLine={false}
            tick={{ fontSize: 11, fill: 'var(--color-text-secondary)' }}
            width={yAxisWidth}
            interval={0}
          />
          <Tooltip
            content={<CustomTooltip />}
            cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
            wrapperStyle={{ transition: 'none' }}
            isAnimationActive={false}
          />
          <Bar dataKey="success" stackId="stack" radius={[2, 2, 2, 2]} isAnimationActive={false}>
            {data.map((entry, index) => (
              <Cell key={`success-${index}`} fill={COLORS[entry.type].success} />
            ))}
          </Bar>
          <Bar dataKey="errors" stackId="stack" radius={[2, 2, 2, 2]} isAnimationActive={false}>
            {data.map((entry, index) => (
              <Cell key={`error-${index}`} fill={COLORS[entry.type].error} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

interface TrendsAgentsAndSkillsCardProps {
  data: TrendsAgentsAndSkillsCardData | null;
}

export function TrendsAgentsAndSkillsCard({ data }: TrendsAgentsAndSkillsCardProps) {
  if (!data) return null;

  const totalInvocations = data.total_agent_invocations + data.total_skill_invocations;
  if (totalInvocations === 0) return null;

  const agentChartData = prepareChartData(data.agent_stats, 'agent');
  const skillChartData = prepareChartData(data.skill_stats, 'skill');

  // Build subtitle
  let subtitle: string;
  if (data.total_agent_invocations > 0 && data.total_skill_invocations > 0) {
    subtitle = `${data.total_agent_invocations.toLocaleString()} agent + ${data.total_skill_invocations.toLocaleString()} skill invocations`;
  } else if (data.total_agent_invocations > 0) {
    subtitle = `${data.total_agent_invocations.toLocaleString()} agent invocations`;
  } else {
    subtitle = `${data.total_skill_invocations.toLocaleString()} skill invocations`;
  }

  return (
    <TrendsCard title="Agents & Skills" icon={UsersIcon} subtitle={subtitle}>
      <div className={styles.sectionHeading}>Agents</div>
      {agentChartData.length > 0 ? (
        <SectionChart data={agentChartData} />
      ) : (
        <div className={styles.emptyMessage}>None</div>
      )}

      <div className={styles.sectionHeading}>Skills</div>
      {skillChartData.length > 0 ? (
        <SectionChart data={skillChartData} />
      ) : (
        <div className={styles.emptyMessage}>None</div>
      )}
    </TrendsCard>
  );
}
