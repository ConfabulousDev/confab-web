import { CardWrapper, StatRow, CardLoading } from './Card';
import { formatCost } from '@/utils/tokenStats';
import type { CostCardData } from '@/schemas/api';
import type { CardProps } from './types';
import styles from '../SessionSummaryPanel.module.css';

const InfoIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
    <path d="M12 16v-4" />
    <path d="M12 8h.01" />
  </svg>
);

const TOOLTIP = 'Estimated API cost based on token usage and model pricing (assumes 5-minute prompt caching)';

export function CostCard({ data, loading }: CardProps<CostCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Cost">
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  const cost = parseFloat(data.estimated_usd);

  return (
    <CardWrapper title="Cost">
      <StatRow
        label="Estimated"
        value={formatCost(cost)}
        icon={InfoIcon}
        tooltip={TOOLTIP}
        valueClassName={styles.cost}
      />
    </CardWrapper>
  );
}
