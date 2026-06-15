import { CardWrapper, StatRow } from './Card';
import { useCardState } from './useCardState';
import { formatResponseTime } from '@/utils/compactionStats';
import { formatModelDisplayName } from '@/utils/formatting';
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
import { prepareBreakdownData, type BreakdownEntry } from './sessionCardBreakdown';
import styles from './SessionCard.module.css';

const TOOLTIPS = {
  totalMessages: 'Total transcript lines in the session',
  assistantMessages: 'All assistant-role messages',
  duration: 'Time from first to last message',
  models: 'AI models used in this session',
  compactionAuto: 'Compactions triggered automatically when context limit reached',
  compactionManual: 'Compactions triggered manually by user',
  compactionAvgTime: 'Average time for server-side summarization (auto compactions only)',
  // userMessages varies by provider (CF-437) — see userMessagesTooltip below.
  userMessagesClaude: 'All user-role messages (human prompts + tool results)',
  userMessagesCodex:
    'User-role messages (human prompts only; tool outputs counted separately)',
};

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

interface SessionCardProps extends CardProps<SessionCardData> {
  /** Session provider (CF-437). Drives reasoning bar label, tool-results bar
   *  visibility, and Messages tooltip wording. */
  provider: string;
}

/**
 * Registry-friendly wrapper. The card registry's generic component type
 * doesn't model `provider`; SessionSummaryPanel injects it at runtime via
 * extraProps. This wrapper defaults provider to "claude-code" if it ever
 * arrives unset — defensive against a runtime hole. Direct callers of
 * SessionCard still get the TS-enforced required prop.
 */
export function SessionCardForRegistry(
  props: Omit<SessionCardProps, 'provider'> & { provider?: string }
) {
  return <SessionCard {...props} provider={props.provider ?? 'claude-code'} />;
}

export function SessionCard({ data, loading, error, provider }: SessionCardProps) {
  const guard = useCardState(data, loading, error, { title: 'Session', icon: TerminalIcon });
  if (guard) return guard;

  if (!data) return null;

  const isCodex = provider === 'codex';
  const userMessagesTooltip = isCodex ? TOOLTIPS.userMessagesCodex : TOOLTIPS.userMessagesClaude;

  const hasCompaction = data.compaction_auto > 0 || data.compaction_manual > 0;
  const breakdownData = prepareBreakdownData(data, provider);

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
          value={data.models_used.map(formatModelDisplayName).join(', ')}
          icon={RobotIcon}
          tooltip={TOOLTIPS.models}
        />
      )}

      {/* Messages */}
      <StatRow
        label="Messages"
        value={`${data.total_messages} (${data.user_messages}/${data.assistant_messages})`}
        icon={ChatIcon}
        tooltip={`${TOOLTIPS.totalMessages}; ${userMessagesTooltip}; ${TOOLTIPS.assistantMessages}`}
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
