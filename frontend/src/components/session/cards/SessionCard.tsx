import { CardWrapper, StatRow, CardLoading } from './Card';
import { formatResponseTime } from '@/utils/compactionStats';
import {
  TerminalIcon,
  ChatIcon,
  DurationIcon,
  RobotIcon,
  CompressIcon,
} from '@/components/icons';
import type { SessionCardData } from '@/schemas/api';
import type { CardProps } from './types';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import styles from './SessionCard.module.css';

const TOOLTIPS = {
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
  fullName: string;
  value: number;
  color: string;
  [key: string]: string | number; // Index signature for Recharts compatibility
}

function prepareBreakdownData(data: SessionCardData): BreakdownEntry[] {
  const entries: BreakdownEntry[] = [
    { name: 'Prompts', fullName: 'Human prompts', value: data.human_prompts, color: BREAKDOWN_COLORS.humanPrompts },
    { name: 'Tool res', fullName: 'Tool results', value: data.tool_results, color: BREAKDOWN_COLORS.toolResults },
    { name: 'Txt resp', fullName: 'Text responses', value: data.text_responses, color: BREAKDOWN_COLORS.textResponses },
    { name: 'Tool calls', fullName: 'Tool calls', value: data.tool_calls, color: BREAKDOWN_COLORS.toolCalls },
    { name: 'Thinking', fullName: 'Thinking blocks', value: data.thinking_blocks, color: BREAKDOWN_COLORS.thinkingBlocks },
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
      <div className={styles.tooltipTitle}>{entry.payload.fullName}</div>
      <div className={styles.tooltipValue}>{entry.value}</div>
    </div>
  );
}

/**
 * Format duration for session overview display.
 *
 * NOTE: This is intentionally simpler than utils/formatting.ts formatDuration.
 * - Shows "5m" not "5m 30s" - session overview doesn't need second-level precision
 * - No millisecond display - sessions are always long enough to show seconds
 *
 * For precise timing display, see ConversationCard or TimelineBar variants.
 */
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

/**
 * Format model name for user-friendly display.
 *
 * NOTE: This differs from utils/formatting.ts formatModelName which returns
 * technical format ("claude-sonnet-4"). This version returns friendly format
 * ("Sonnet 4") for the session card UI.
 *
 * Examples:
 *   "claude-sonnet-4-20241022" -> "Sonnet 4"
 *   "claude-opus-4-5-20251101" -> "Opus 4.5"
 */
function formatModelName(model: string): string {
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
      <CardWrapper title="Session" icon={TerminalIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  const hasCompaction = data.compaction_auto > 0 || data.compaction_manual > 0;
  const breakdownData = prepareBreakdownData(data);

  return (
    <CardWrapper title="Session" icon={TerminalIcon}>
      {/* Duration */}
      {data.duration_ms != null && (
        <StatRow
          label="Duration"
          value={formatDuration(data.duration_ms)}
          icon={DurationIcon}
          tooltip={TOOLTIPS.duration}
        />
      )}

      {/* Models */}
      {data.models_used.length > 0 && (
        <StatRow
          label={data.models_used.length === 1 ? 'Model' : 'Models'}
          value={data.models_used.map(formatModelName).join(', ')}
          icon={RobotIcon}
          tooltip={TOOLTIPS.models}
        />
      )}

      {/* Messages */}
      <StatRow
        label="Messages"
        value={`${data.total_messages} (${data.user_messages}/${data.assistant_messages})`}
        icon={ChatIcon}
        tooltip={`${TOOLTIPS.totalMessages}; ${TOOLTIPS.userMessages}; ${TOOLTIPS.assistantMessages}`}
      />

      {/* Message type breakdown bar chart */}
      {breakdownData.length > 0 && (() => {
        // Calculate dynamic YAxis width based on longest label (~6.5px per char at 11px font)
        const maxLabelLength = Math.max(...breakdownData.map((d) => d.name.length));
        const yAxisWidth = Math.max(80, maxLabelLength * 6.5 + 8);
        return (
          <div className={styles.chartContainer} style={{ height: breakdownData.length * 24 + 16 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={breakdownData}
                layout="vertical"
                margin={{ top: 0, right: 16, left: 0, bottom: 0 }}
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
                  tick={{ fontSize: 11, fill: 'var(--color-text-secondary)' }}
                  width={yAxisWidth}
                  interval={0}
                />
                <Tooltip
                  content={<CustomTooltip />}
                  cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
                />
                <Bar dataKey="value" radius={[2, 2, 2, 2]} isAnimationActive={false}>
                  {breakdownData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        );
      })()}

      {/* Compaction stats */}
      {hasCompaction && (
        <>
          <StatRow
            label="Compactions"
            value={`${data.compaction_auto + data.compaction_manual} (${data.compaction_manual}/${data.compaction_auto})`}
            icon={CompressIcon}
            tooltip={`Total compactions (manual/auto); ${TOOLTIPS.compactionManual}; ${TOOLTIPS.compactionAuto}`}
          />
          {data.compaction_avg_time_ms != null && (
            <StatRow
              label="Avg time (auto)"
              value={formatResponseTime(data.compaction_avg_time_ms)}
              icon={DurationIcon}
              tooltip={TOOLTIPS.compactionAvgTime}
            />
          )}
        </>
      )}
    </CardWrapper>
  );
}
