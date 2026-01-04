import { CardWrapper, StatRow, CardLoading } from './Card';
import {
  ConversationIcon,
  RefreshIcon,
  DurationIcon,
  UserIcon,
  ZapIcon,
} from '@/components/icons';
import type { ConversationCardData } from '@/schemas/api';
import type { CardProps } from './types';
import styles from '../SessionSummaryPanel.module.css';

const TOOLTIPS = {
  userPrompts: 'Number of user prompts in the conversation',
  avgClaudeTime: 'Average time Claude spent responding per prompt',
  avgUserTime: 'Average time between Claude finishing and user responding',
  totalClaudeTime: 'Total time Claude spent working across all prompts',
  totalUserTime: 'Total time user spent between prompts',
  claudeUtilization: 'Percentage of session time Claude was actively working',
};

/**
 * Format duration for conversation timing display.
 *
 * NOTE: This variant differs from utils/formatting.ts and SessionCard:
 * - Shows "5m 30s" (includes seconds for timing precision)
 * - Shows "500ms" for sub-second durations (useful for response times)
 *
 * Used for Claude/user turn times where precision matters.
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
    const remainingSeconds = seconds % 60;
    return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
  }
  if (seconds > 0) {
    return `${seconds}s`;
  }
  return `${ms}ms`;
}

export function ConversationCard({ data, loading }: CardProps<ConversationCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Conversation" icon={ConversationIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  return (
    <CardWrapper title="Conversation" icon={ConversationIcon}>
      {data.assistant_utilization != null && (
        <StatRow
          label="Claude utilization"
          value={`${data.assistant_utilization.toFixed(0)}%`}
          icon={ZapIcon}
          tooltip={TOOLTIPS.claudeUtilization}
          valueClassName={styles.utilization}
        />
      )}
      {data.total_assistant_duration_ms != null && (
        <StatRow
          label="Total Claude time"
          value={formatDuration(data.total_assistant_duration_ms)}
          icon={DurationIcon}
          tooltip={TOOLTIPS.totalClaudeTime}
        />
      )}
      {data.total_user_duration_ms != null && (
        <StatRow
          label="Total user time"
          value={formatDuration(data.total_user_duration_ms)}
          icon={UserIcon}
          tooltip={TOOLTIPS.totalUserTime}
        />
      )}
      <StatRow
        label="User prompts"
        value={data.user_turns}
        icon={RefreshIcon}
        tooltip={TOOLTIPS.userPrompts}
      />
      {data.avg_assistant_turn_ms != null && (
        <StatRow
          label="Avg Claude time"
          value={formatDuration(data.avg_assistant_turn_ms)}
          icon={DurationIcon}
          tooltip={TOOLTIPS.avgClaudeTime}
        />
      )}
      {data.avg_user_thinking_ms != null && (
        <StatRow
          label="Avg user time"
          value={formatDuration(data.avg_user_thinking_ms)}
          icon={UserIcon}
          tooltip={TOOLTIPS.avgUserTime}
        />
      )}
    </CardWrapper>
  );
}
