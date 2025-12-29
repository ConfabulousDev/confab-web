import { CardWrapper, StatRow, CardLoading, SectionHeader } from './Card';
import { formatResponseTime } from '@/utils/compactionStats';
import type { SessionCardData } from '@/schemas/api';
import type { CardProps } from './types';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts';
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

// Colors for the pie chart segments
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
  // Filter out zero values
  return entries.filter((e) => e.value > 0);
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
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

      {/* Message type breakdown pie chart */}
      {breakdownData.length > 0 && (
        <>
          <SectionHeader label="Breakdown" />
          <div className={styles.chartContainer}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={breakdownData}
                  dataKey="value"
                  nameKey="name"
                  cx="50%"
                  cy="50%"
                  innerRadius={30}
                  outerRadius={55}
                  paddingAngle={2}
                >
                  {breakdownData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip content={<CustomTooltip />} />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className={styles.legend}>
            {breakdownData.map((entry) => (
              <div key={entry.name} className={styles.legendItem}>
                <span className={styles.legendDot} style={{ backgroundColor: entry.color }} />
                <span>
                  {entry.name} ({entry.value})
                </span>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Compaction section */}
      {hasCompaction && (
        <>
          <SectionHeader label="Compaction" />
          <StatRow label="Auto" value={data.compaction_auto} tooltip={TOOLTIPS.compactionAuto} />
          <StatRow
            label="Manual"
            value={data.compaction_manual}
            tooltip={TOOLTIPS.compactionManual}
          />
          <StatRow
            label="Avg time"
            value={formatResponseTime(data.compaction_avg_time_ms ?? null)}
            tooltip={TOOLTIPS.compactionAvgTime}
          />
        </>
      )}
    </CardWrapper>
  );
}
