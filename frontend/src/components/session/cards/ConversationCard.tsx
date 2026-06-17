import { CardWrapper, StatRow } from './Card';
import { useCardState } from './useCardState';
import {
  ConversationIcon,
  RefreshIcon,
  DurationIcon,
  UserIcon,
  ZapIcon,
} from '@/components/icons';
import type { ConversationCardData } from '@/schemas/api';
import type { CardProps } from './types';
import { providerLabel } from '@/utils/providers';
import { formatTokenSpeed } from '@/utils/tokenStats';
import { getAdapter, isTokensMeasurable } from '@/providers/registry';
import styles from '../SessionSummaryPanel.module.css';

const TOKEN_SPEED_TOOLTIP = 'Output tokens generated per second (output ÷ assistant time)';

/**
 * Format duration for conversation timing display.
 *
 * NOTE: This variant differs from utils/formatting.ts and SessionCard:
 * - Shows "5m 30s" (includes seconds for timing precision)
 * - Shows "500ms" for sub-second durations (useful for response times)
 *
 * Used for assistant/user turn times where precision matters.
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

/**
 * Props for ConversationCard. `provider` is required so tooltips render the
 * correct agent name (e.g. "Average time Codex spent responding…"). Labels
 * stay provider-neutral ("Assistant utilization") to keep stat rows compact.
 */
interface ConversationCardProps extends CardProps<ConversationCardData> {
  provider: string;
  /**
   * CF-525: precomputed output-tokens-per-second for the session. Computed by
   * SessionSummaryPanel (the only place holding both the Tokens and
   * Conversation card data); `null` when timing/output is unavailable, which
   * renders as "—". The card stays presentational and does no token math.
   */
  tokenSpeed?: number | null;
}

/**
 * Registry-friendly wrapper. The card registry's generic component type
 * doesn't model `provider`; SessionSummaryPanel injects it at runtime via
 * extraProps. This wrapper defaults provider to "claude-code" if it ever
 * arrives unset — defensive against a runtime hole. Direct callers of
 * ConversationCard still get the TS-enforced required prop.
 */
export function ConversationCardForRegistry(
  props: Omit<ConversationCardProps, 'provider'> & { provider?: string }
) {
  return <ConversationCard {...props} provider={props.provider ?? 'claude-code'} />;
}

export function ConversationCard({ data, loading, error, provider, tokenSpeed }: ConversationCardProps) {
  const guard = useCardState(data, loading, error, { title: 'Conversation', icon: ConversationIcon });
  if (guard) return guard;

  if (!data) return null;

  const adapter = getAdapter(provider);
  const agent = providerLabel(provider);
  const measurable = isTokensMeasurable(provider);
  const tooltips = {
    userPrompts: 'Number of user prompts in the conversation',
    avgAssistantTime: `Average time ${agent} spent responding per prompt`,
    avgUserTime: `Average time between ${agent} finishing and user responding`,
    totalAssistantTime: `Total time ${agent} spent working across all prompts`,
    totalUserTime: 'Total time user spent between prompts',
    assistantUtilization: `Percentage of session time ${agent} was actively working`,
    tokenSpeed:
      !measurable && tokenSpeed == null
        ? (adapter.tokenSpeedUnavailableTooltip ?? TOKEN_SPEED_TOOLTIP)
        : TOKEN_SPEED_TOOLTIP,
  };

  return (
    <CardWrapper title="Conversation" icon={ConversationIcon}>
      {data.assistant_utilization_pct != null && (
        <StatRow
          label="Assistant utilization"
          value={`${data.assistant_utilization_pct.toFixed(0)}%`}
          icon={ZapIcon}
          tooltip={tooltips.assistantUtilization}
          valueClassName={styles.utilization}
        />
      )}
      {data.total_assistant_duration_ms != null && (
        <StatRow
          label="Total assistant time"
          value={formatDuration(data.total_assistant_duration_ms)}
          icon={DurationIcon}
          tooltip={tooltips.totalAssistantTime}
        />
      )}
      <StatRow
        label="Token speed"
        value={formatTokenSpeed(tokenSpeed ?? null)}
        icon={ZapIcon}
        tooltip={tooltips.tokenSpeed}
      />
      {data.total_user_duration_ms != null && (
        <StatRow
          label="Total user time"
          value={formatDuration(data.total_user_duration_ms)}
          icon={UserIcon}
          tooltip={tooltips.totalUserTime}
        />
      )}
      <StatRow
        label="User prompts"
        value={data.user_turns}
        icon={RefreshIcon}
        tooltip={tooltips.userPrompts}
      />
      {data.avg_assistant_turn_ms != null && (
        <StatRow
          label="Avg assistant time"
          value={formatDuration(data.avg_assistant_turn_ms)}
          icon={DurationIcon}
          tooltip={tooltips.avgAssistantTime}
        />
      )}
      {data.avg_user_thinking_ms != null && (
        <StatRow
          label="Avg user time"
          value={formatDuration(data.avg_user_thinking_ms)}
          icon={UserIcon}
          tooltip={tooltips.avgUserTime}
        />
      )}
    </CardWrapper>
  );
}
