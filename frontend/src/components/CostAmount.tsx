import { formatCost } from '@/utils/tokenStats';
import { cx } from '@/utils/utils';
import styles from './CostAmount.module.css';

interface CostAmountProps {
  /** Cost in USD. Call sites parse wire strings (`parseFloat(cost_usd)`). */
  usd: number;
  /** Extra classes for layout (weight, nowrap, alignment) — color is owned here. */
  className?: string;
}

/**
 * fdp3: the single shared renderer for every displayed dollar amount. Drives the
 * money green off the `--color-cost` theme token (light + dark) and centralizes
 * the $0 rule in one place: an exact `$0.00` (unpriced / cost unavailable) shifts
 * to the warning color, while a tiny non-zero `<$0.01` stays green.
 */
export function CostAmount({ usd, className }: CostAmountProps) {
  const isZero = usd === 0;
  return (
    <span className={cx(styles.cost, isZero && styles.zero, className)}>
      {formatCost(usd)}
    </span>
  );
}
