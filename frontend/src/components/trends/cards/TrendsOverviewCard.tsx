import { TrendsCard, StatRow } from './TrendsCard';
import { SparklesIcon, DurationIcon, CalendarIcon, RobotIcon, ZapIcon } from '@/components/icons';
import type { TrendsOverviewCard as TrendsOverviewCardData } from '@/schemas/api';
import { formatDuration } from '@/utils';
import { formatTokenSpeed } from '@/utils/tokenStats';

interface TrendsOverviewCardProps {
  data: TrendsOverviewCardData | null;
  /**
   * CF-525: precomputed aggregate output-tokens-per-second over the range.
   * Computed by TrendsPage (the only place holding both the overview's
   * assistant duration and the tokens card's output count); `null` when the
   * range has no assistant time. Rendered as "-" to match sibling empty rows.
   */
  tokenSpeed?: number | null;
}

export function TrendsOverviewCard({ data, tokenSpeed }: TrendsOverviewCardProps) {
  if (!data) return null;

  // Match the card's local "-" empty convention rather than the helper's "—".
  const tokenSpeedDisplay = tokenSpeed != null ? formatTokenSpeed(tokenSpeed) : '-';

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
      icon={SparklesIcon}
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
        label="Token Speed"
        value={tokenSpeedDisplay}
        icon={ZapIcon}
      />
      <StatRow
        label="Utilization"
        value={utilization}
        icon={ZapIcon}
      />
    </TrendsCard>
  );
}
