import { CardWrapper, StatRow, CardLoading, SectionHeader } from './Card';
import {
  ConversationIcon,
  RefreshIcon,
  DurationIcon,
  ThinkingIcon,
  ZapIcon,
} from '@/components/icons';
import type { ConversationCardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  userTurns: 'Number of user prompts in the conversation',
  assistantTurns: 'Number of assistant text responses',
  avgAssistantTurn: 'Average time Claude spent per turn (including tool calls)',
  avgUserThinking: 'Average time between Claude finishing and user responding',
  totalAssistantDuration: 'Total time Claude spent working across all turns',
  totalUserDuration: 'Total time user spent thinking between turns',
  assistantUtilization: 'Percentage of session time Claude was actively working',
};

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

  const hasTiming =
    data.avg_assistant_turn_ms != null ||
    data.avg_user_thinking_ms != null ||
    data.total_assistant_duration_ms != null ||
    data.total_user_duration_ms != null ||
    data.assistant_utilization != null;

  return (
    <CardWrapper title="Conversation" icon={ConversationIcon}>
      {/* Turn counts */}
      <StatRow
        label="User turns"
        value={data.user_turns}
        icon={RefreshIcon}
        tooltip={TOOLTIPS.userTurns}
      />
      <StatRow
        label="Assistant turns"
        value={data.assistant_turns}
        icon={RefreshIcon}
        tooltip={TOOLTIPS.assistantTurns}
      />

      {/* Timing metrics */}
      {hasTiming && (
        <>
          <SectionHeader label="Timing" />
          {data.total_assistant_duration_ms != null && (
            <StatRow
              label="Total Claude time"
              value={formatDuration(data.total_assistant_duration_ms)}
              icon={DurationIcon}
              tooltip={TOOLTIPS.totalAssistantDuration}
            />
          )}
          {data.total_user_duration_ms != null && (
            <StatRow
              label="Total user time"
              value={formatDuration(data.total_user_duration_ms)}
              icon={ThinkingIcon}
              tooltip={TOOLTIPS.totalUserDuration}
            />
          )}
          {data.assistant_utilization != null && (
            <StatRow
              label="Claude utilization"
              value={`${data.assistant_utilization.toFixed(0)}%`}
              icon={ZapIcon}
              tooltip={TOOLTIPS.assistantUtilization}
            />
          )}
          {data.avg_assistant_turn_ms != null && (
            <StatRow
              label="Avg Claude turn"
              value={formatDuration(data.avg_assistant_turn_ms)}
              icon={DurationIcon}
              tooltip={TOOLTIPS.avgAssistantTurn}
            />
          )}
          {data.avg_user_thinking_ms != null && (
            <StatRow
              label="Avg user thinking"
              value={formatDuration(data.avg_user_thinking_ms)}
              icon={ThinkingIcon}
              tooltip={TOOLTIPS.avgUserThinking}
            />
          )}
        </>
      )}
    </CardWrapper>
  );
}
