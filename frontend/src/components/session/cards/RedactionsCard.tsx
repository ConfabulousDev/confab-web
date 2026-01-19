import { CardWrapper, StatRow, CardLoading, CardError } from './Card';
import { ShieldIcon } from '@/components/icons';
import type { RedactionsCardData } from '@/schemas/api';
import type { CardProps } from './types';

/**
 * Displays counts of redacted secrets by type.
 * Hidden entirely if no redactions were found.
 */
export function RedactionsCard({ data, loading, error }: CardProps<RedactionsCardData>) {
  if (error && !data) {
    return <CardError title="Redactions" error={error} icon={ShieldIcon} />;
  }

  if (loading && !data) {
    return (
      <CardWrapper title="Redactions" icon={ShieldIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  // Don't render the card if no redactions were found
  if (data.total_redactions === 0) return null;

  // Sort by count descending
  const sortedEntries = Object.entries(data.redaction_counts).sort(
    ([, a], [, b]) => b - a
  );

  const subtitle = `${data.total_redactions} total`;

  return (
    <CardWrapper title="Redactions" icon={ShieldIcon} subtitle={subtitle}>
      {sortedEntries.map(([type, count]) => (
        <StatRow
          key={type}
          label={type}
          value={count}
          tooltip={`${count} occurrence${count !== 1 ? 's' : ''} of [REDACTED:${type}]`}
        />
      ))}
    </CardWrapper>
  );
}
