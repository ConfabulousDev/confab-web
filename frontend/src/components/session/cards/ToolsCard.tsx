import { CardWrapper, StatRow, CardLoading } from './Card';
import type { ToolsCardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  totalCalls: 'Total number of tool invocations',
  topTools: 'Most frequently used tools',
  errors: 'Tool calls that returned errors',
};

function getTopTools(breakdown: Record<string, number>, limit = 3): string {
  const sorted = Object.entries(breakdown)
    .sort(([, a], [, b]) => b - a)
    .slice(0, limit);

  if (sorted.length === 0) return 'None';

  return sorted.map(([name, count]) => `${name} (${count})`).join(', ');
}

export function ToolsCard({ data, loading }: CardProps<ToolsCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Tools">
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  // Don't render the card if no tools were used
  if (data.total_calls === 0) return null;

  return (
    <CardWrapper title="Tools">
      <StatRow label="Total calls" value={data.total_calls} tooltip={TOOLTIPS.totalCalls} />
      <StatRow
        label="Top tools"
        value={getTopTools(data.tool_breakdown)}
        tooltip={TOOLTIPS.topTools}
      />
      {data.error_count > 0 && (
        <StatRow label="Errors" value={data.error_count} tooltip={TOOLTIPS.errors} />
      )}
    </CardWrapper>
  );
}
