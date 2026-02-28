import { CardWrapper, StatRow, CardLoading, CardError } from './Card';
import { ZapIcon, DollarIcon } from '@/components/icons';
import { formatCost } from '@/utils/tokenStats';
import type { FastModeCardData } from '@/schemas/api';
import type { CardProps } from './types';

const TOOLTIPS = {
  fastTurns: 'Number of assistant turns using fast mode (6x token cost multiplier)',
  standardTurns: 'Number of assistant turns using standard speed',
  fastCost: 'Total API cost of fast mode turns (includes 6x multiplier)',
  standardCost: 'Total API cost of standard speed turns',
};

export function FastModeCard({ data, loading, error }: CardProps<FastModeCardData>) {
  if (error && !data) {
    return <CardError title="Fast Mode" error={error} icon={ZapIcon} />;
  }

  if (loading && !data) {
    return (
      <CardWrapper title="Fast Mode" icon={ZapIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;
  if (data.fast_turns === 0) return null;

  const totalTurns = data.fast_turns + data.standard_turns;
  const pct = totalTurns > 0 ? Math.round((data.fast_turns / totalTurns) * 100) : 0;

  return (
    <CardWrapper title="Fast Mode" icon={ZapIcon} subtitle={`${pct}% of turns`}>
      <StatRow
        label="Fast turns"
        value={data.fast_turns}
        icon={ZapIcon}
        tooltip={TOOLTIPS.fastTurns}
      />
      <StatRow
        label="Standard turns"
        value={data.standard_turns}
        tooltip={TOOLTIPS.standardTurns}
      />
      <StatRow
        label="Fast mode cost"
        value={formatCost(parseFloat(data.fast_cost_usd))}
        icon={DollarIcon}
        tooltip={TOOLTIPS.fastCost}
      />
      <StatRow
        label="Standard cost"
        value={formatCost(parseFloat(data.standard_cost_usd))}
        icon={DollarIcon}
        tooltip={TOOLTIPS.standardCost}
      />
    </CardWrapper>
  );
}
