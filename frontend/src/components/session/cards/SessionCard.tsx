import { CardWrapper, StatRow, CardLoading } from './Card';
import { formatResponseTime } from '@/utils/compactionStats';
import type { SessionCardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  userTurns: 'Number of messages you sent',
  assistantTurns: 'Number of responses from Claude',
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

  const totalTurns = data.user_turns + data.assistant_turns;
  const hasCompaction = data.compaction_auto > 0 || data.compaction_manual > 0;

  return (
    <CardWrapper title="Session">
      <StatRow label="Total turns" value={totalTurns} tooltip={TOOLTIPS.userTurns} />
      {data.duration_ms != null && (
        <StatRow
          label="Duration"
          value={formatDuration(data.duration_ms)}
          tooltip={TOOLTIPS.duration}
        />
      )}
      {data.models_used.length > 0 && (
        <StatRow
          label={data.models_used.length === 1 ? 'Model' : 'Models'}
          value={data.models_used.map(formatModelName).join(', ')}
          tooltip={TOOLTIPS.models}
        />
      )}
      {hasCompaction && (
        <>
          <StatRow
            label="Compaction (auto)"
            value={data.compaction_auto}
            tooltip={TOOLTIPS.compactionAuto}
          />
          <StatRow
            label="Compaction (manual)"
            value={data.compaction_manual}
            tooltip={TOOLTIPS.compactionManual}
          />
          <StatRow
            label="Avg compaction time"
            value={formatResponseTime(data.compaction_avg_time_ms ?? null)}
            tooltip={TOOLTIPS.compactionAvgTime}
          />
        </>
      )}
    </CardWrapper>
  );
}
