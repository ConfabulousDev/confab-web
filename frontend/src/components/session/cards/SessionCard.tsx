import { CardWrapper, StatRow, CardLoading, SectionHeader } from './Card';
import { formatResponseTime } from '@/utils/compactionStats';
import type { SessionCardData } from '@/schemas/api';
import type { CardProps } from './types';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import styles from './SessionCard.module.css';

const TOOLTIPS = {
  turns: 'Actual conversational exchanges (user prompts and text responses)',
  totalMessages: 'Total transcript lines in the session',
  userMessages: 'All user-role messages (human prompts + tool results)',
  assistantMessages: 'All assistant-role messages',
  duration: 'Time from first to last message',
  models: 'AI models used in this session',
  compactionAuto: 'Compactions triggered automatically when context limit reached',
  compactionManual: 'Compactions triggered manually by user',
  compactionAvgTime: 'Average time for server-side summarization (auto compactions only)',
};

// Colors for the bar chart
const BREAKDOWN_COLORS = {
  humanPrompts: '#3b82f6', // blue
  toolResults: '#8b5cf6', // purple
  textResponses: '#22c55e', // green
  toolCalls: '#f59e0b', // amber
  thinkingBlocks: '#ec4899', // pink
};

interface BreakdownEntry {
  name: string;
  value: number;
  color: string;
  [key: string]: string | number; // Index signature for Recharts compatibility
}

function prepareBreakdownData(data: SessionCardData): BreakdownEntry[] {
  const entries: BreakdownEntry[] = [
    { name: 'Human prompts', value: data.human_prompts, color: BREAKDOWN_COLORS.humanPrompts },
    { name: 'Tool results', value: data.tool_results, color: BREAKDOWN_COLORS.toolResults },
    { name: 'Text responses', value: data.text_responses, color: BREAKDOWN_COLORS.textResponses },
    { name: 'Tool calls', value: data.tool_calls, color: BREAKDOWN_COLORS.toolCalls },
    { name: 'Thinking', value: data.thinking_blocks, color: BREAKDOWN_COLORS.thinkingBlocks },
  ];
  // Filter out zero values and sort by value descending
  return entries.filter((e) => e.value > 0).sort((a, b) => b.value - a.value);
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    value: number;
    payload: BreakdownEntry;
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const entry = payload[0];
  if (!entry) return null;

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{entry.payload.name}</div>
      <div className={styles.tooltipValue}>{entry.value}</div>
    </div>
  );
}

function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
}

function formatModelName(model: string): string {
  // Extract the model family and version for display
  // e.g., "claude-sonnet-4-20241022" -> "Sonnet 4"
  // e.g., "claude-opus-4-5-20251101" -> "Opus 4.5"
  const match = model.match(/claude-(\w+)-(\d+)(?:-(\d+))?/);
  if (match) {
    const family = match[1];
    const major = match[2];
    const minor = match[3];
    if (family && major) {
      const familyName = family.charAt(0).toUpperCase() + family.slice(1);
      const version = minor ? `${major}.${minor}` : major;
      return `${familyName} ${version}`;
    }
  }
  return model;
}

export function SessionCard({ data, loading }: CardProps<SessionCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Session">
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  const hasCompaction = data.compaction_auto > 0 || data.compaction_manual > 0;
  const breakdownData = prepareBreakdownData(data);

  return (
    <CardWrapper title="Session">
      {/* Messages */}
      <StatRow
        label="Messages"
        value={`${data.total_messages} (${data.user_messages}/${data.assistant_messages})`}
        tooltip={`${TOOLTIPS.totalMessages}; ${TOOLTIPS.userMessages}; ${TOOLTIPS.assistantMessages}`}
      />

      {/* Turns */}
      <StatRow
        label="Turns"
        value={`${data.user_turns + data.assistant_turns} (${data.user_turns}/${data.assistant_turns})`}
        tooltip={TOOLTIPS.turns}
      />

      {/* Duration */}
      {data.duration_ms != null && (
        <StatRow
          label="Duration"
          value={formatDuration(data.duration_ms)}
          tooltip={TOOLTIPS.duration}
        />
      )}

      {/* Models */}
      {data.models_used.length > 0 && (
        <StatRow
          label={data.models_used.length === 1 ? 'Model' : 'Models'}
          value={data.models_used.map(formatModelName).join(', ')}
          tooltip={TOOLTIPS.models}
        />
      )}

      {/* Message type breakdown bar chart */}
      {breakdownData.length > 0 && (() => {
        // Calculate dynamic YAxis width based on longest label (~6.5px per char at 11px font)
        const maxLabelLength = Math.max(...breakdownData.map((d) => d.name.length));
        const yAxisWidth = Math.max(80, maxLabelLength * 6.5 + 8);
        return (
        <>
          <SectionHeader label="Breakdown" />
          <div className={styles.chartContainer} style={{ height: breakdownData.length * 20 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={breakdownData}
                layout="vertical"
                margin={{ top: 0, right: 8, left: 0, bottom: 0 }}
                barSize={12}
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
                  tick={{ fontSize: 11, fill: 'var(--color-text-secondary)', style: { whiteSpace: 'nowrap' } }}
                  width={yAxisWidth}
                />
                <Tooltip
                  content={<CustomTooltip />}
                  cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
                />
                <Bar dataKey="value" radius={[2, 2, 2, 2]}>
                  {breakdownData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </>
        );
      })()}

      {/* Compaction stats */}
      {hasCompaction && (
        <>
          <StatRow
            label="Compactions"
            value={`${data.compaction_auto + data.compaction_manual} (${data.compaction_manual}/${data.compaction_auto})`}
            tooltip={`Total compactions (manual/auto); ${TOOLTIPS.compactionManual}; ${TOOLTIPS.compactionAuto}`}
          />
          {data.compaction_avg_time_ms != null && (
            <StatRow
              label="Avg time (auto)"
              value={formatResponseTime(data.compaction_avg_time_ms)}
              tooltip={TOOLTIPS.compactionAvgTime}
            />
          )}
        </>
      )}
    </CardWrapper>
  );
}
