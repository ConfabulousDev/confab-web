import { CardWrapper, StatRow, CardLoading } from './Card';
import { formatResponseTime } from '@/utils/compactionStats';
import type { CompactionCardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  auto: 'Compactions triggered automatically when context limit reached',
  manual: 'Compactions triggered manually by user',
  avgTime: 'Average time for server-side summarization (auto compactions only)',
};

export function CompactionCard({ data, loading }: CardProps<CompactionCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Compaction">
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  return (
    <CardWrapper title="Compaction">
      <StatRow
        label="Auto"
        value={data.auto}
        tooltip={TOOLTIPS.auto}
      />
      <StatRow
        label="Manual"
        value={data.manual}
        tooltip={TOOLTIPS.manual}
      />
      <StatRow
        label="Avg time (auto)"
        value={formatResponseTime(data.avg_time_ms ?? null)}
        tooltip={TOOLTIPS.avgTime}
      />
    </CardWrapper>
  );
}
