import { TrendsCard, StatRow } from './TrendsCard';
import { ClaudeCodeIcon, DurationIcon, CalendarIcon, RobotIcon, ZapIcon } from '@/components/icons';
import type { TrendsOverviewCard as TrendsOverviewCardData } from '@/schemas/api';
import { formatDuration } from '@/utils';

interface TrendsOverviewCardProps {
  data: TrendsOverviewCardData | null;
}

export function TrendsOverviewCard({ data }: TrendsOverviewCardProps) {
  if (!data) return null;

  const totalDuration = data.total_duration_ms > 0
    ? formatDuration(data.total_duration_ms)
    : '-';

  const avgDuration = data.avg_duration_ms
    ? formatDuration(data.avg_duration_ms)
    : '-';

  const assistantDuration = data.total_assistant_duration_ms > 0
    ? formatDuration(data.total_assistant_duration_ms)
    : '-';

  const utilization = data.assistant_utilization_pct != null
    ? `${data.assistant_utilization_pct.toFixed(1)}%`
    : '-';

  return (
    <TrendsCard
      title="Overview"
      icon={ClaudeCodeIcon}
      subtitle={`${data.days_covered} day${data.days_covered !== 1 ? 's' : ''} with activity`}
    >
      <StatRow
        label="Sessions"
        value={data.session_count.toLocaleString()}
      />
      <StatRow
        label="Total Time"
        value={totalDuration}
        icon={DurationIcon}
      />
      <StatRow
        label="Avg Session"
        value={avgDuration}
        icon={CalendarIcon}
      />
      <StatRow
        label="Total Assistant Time"
        value={assistantDuration}
        icon={RobotIcon}
      />
      <StatRow
        label="Utilization"
        value={utilization}
        icon={ZapIcon}
      />
    </TrendsCard>
  );
}
