import { CardWrapper, StatRow, CardLoading, SectionHeader } from './Card';
import { formatResponseTime } from '@/utils/compactionStats';
import type { SessionCardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  turns: 'Actual conversational exchanges (user prompts and text responses)',
  totalMessages: 'Total transcript lines in the session',
  userMessages: 'All user-role messages (human prompts + tool results)',
  assistantMessages: 'All assistant-role messages',
  humanPrompts: 'User messages with human-typed content',
  toolResults: 'User messages containing tool execution results',
  textResponses: 'Assistant messages containing text output',
  toolCalls: 'Assistant messages with only tool calls (no text output)',
  thinkingBlocks: 'Assistant messages with only thinking (no text output)',
  duration: 'Time from first to last message',
  models: 'AI models used in this session',
  compactionAuto: 'Compactions triggered automatically when context limit reached',
  compactionManual: 'Compactions triggered manually by user',
  compactionAvgTime: 'Average time for server-side summarization (auto compactions only)',
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

  return (
    <CardWrapper title="Session">
      {/* Turns - the key metric */}
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

      {/* Message breakdown section */}
      <SectionHeader label="Messages" />
      <StatRow
        label="Total"
        value={`${data.total_messages} (${data.user_messages}/${data.assistant_messages})`}
        tooltip={`${TOOLTIPS.totalMessages}; ${TOOLTIPS.userMessages}; ${TOOLTIPS.assistantMessages}`}
      />

      {/* Message type breakdown */}
      <SectionHeader label="Breakdown" />
      <StatRow label="Human prompts" value={data.human_prompts} tooltip={TOOLTIPS.humanPrompts} />
      <StatRow label="Tool results" value={data.tool_results} tooltip={TOOLTIPS.toolResults} />
      <StatRow label="Text responses" value={data.text_responses} tooltip={TOOLTIPS.textResponses} />
      <StatRow label="Tool calls" value={data.tool_calls} tooltip={TOOLTIPS.toolCalls} />
      <StatRow
        label="Thinking blocks"
        value={data.thinking_blocks}
        tooltip={TOOLTIPS.thinkingBlocks}
      />

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
