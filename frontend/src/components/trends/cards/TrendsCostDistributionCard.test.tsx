import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TrendsCostDistributionCard } from './TrendsCostDistributionCard';
import type {
  TrendsCostDistributionCard as TrendsCostDistributionCardData,
  TrendsCostDistributionBucket,
} from '@/schemas/api';

// Representative dynamic log10 bands (floor + decades up to $10–$100).
const LABELS = ['< $0.01', '$0.01 – $0.10', '$0.10 – $1', '$1 – $10', '$10 – $100'];

function buckets(counts: number[], totals: string[]): TrendsCostDistributionBucket[] {
  return LABELS.map((label, i) => ({
    label,
    lo: i,
    hi: i < 4 ? i + 1 : null,
    session_count: counts[i] ?? 0,
    total_usd: totals[i] ?? '0',
  }));
}

function makeData(
  overrides: Partial<TrendsCostDistributionCardData> = {},
): TrendsCostDistributionCardData {
  return {
    buckets: buckets([1, 2, 3, 2, 1], ['0.005', '0.06', '1.50', '12.00', '50.00']),
    percentiles: { p50: '0.50', p90: '12.50', p99: '48.00' },
    covered_session_count: 9,
    total_session_count: 12,
    timed_out: false,
    ...overrides,
  };
}

describe('TrendsCostDistributionCard', () => {
  it('renders each band label with always-visible total-$ labels (no hover)', () => {
    render(<TrendsCostDistributionCard data={makeData()} />);
    for (const label of LABELS) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    // Per-bar total $ is rendered up front, not hover-gated.
    expect(screen.getByText('$1.50')).toBeInTheDocument();
    expect(screen.getByText('$50.00')).toBeInTheDocument();
    // A tiny non-zero band total floors to "<$0.01".
    expect(screen.getByText('<$0.01')).toBeInTheDocument();
  });

  it('abbreviates large band totals compactly ($M)', () => {
    render(
      <TrendsCostDistributionCard
        data={makeData({
          buckets: buckets([1, 0, 0, 0, 1], ['0.005', '0', '0', '0', '2100000.00']),
        })}
      />,
    );
    expect(screen.getByText('$2.1M')).toBeInTheDocument();
  });

  it('renders p50/p90/p99 percentile tiles with formatted costs', () => {
    render(<TrendsCostDistributionCard data={makeData()} />);
    expect(screen.getByText('p50')).toBeInTheDocument();
    expect(screen.getByText('p90')).toBeInTheDocument();
    expect(screen.getByText('p99')).toBeInTheDocument();
    expect(screen.getByText('$0.50')).toBeInTheDocument();
    expect(screen.getByText('$12.50')).toBeInTheDocument();
    expect(screen.getByText('$48.00')).toBeInTheDocument();
  });

  it('hides the percentile tiles when percentiles is null', () => {
    render(<TrendsCostDistributionCard data={makeData({ percentiles: null })} />);
    expect(screen.queryByText('p50')).not.toBeInTheDocument();
    // Bars still render.
    expect(screen.getByText('$10 – $100')).toBeInTheDocument();
  });

  it('renders the coverage + backfill caption', () => {
    render(
      <TrendsCostDistributionCard
        data={makeData({ covered_session_count: 9, total_session_count: 12 })}
      />,
    );
    expect(
      screen.getByText(/Covers 9 of 12 sessions with cost data; percentiles reflect this subset/i),
    ).toBeInTheDocument();
  });

  it('renders a timed-out notice instead of a histogram when timed_out is true', () => {
    render(
      <TrendsCostDistributionCard
        data={makeData({ buckets: [], percentiles: null, covered_session_count: 0, total_session_count: 0, timed_out: true })}
      />,
    );
    expect(screen.getByText(/narrow/i)).toBeInTheDocument();
    expect(screen.queryByText('p50')).not.toBeInTheDocument();
  });

  it('renders nothing when no sessions carry cost data (covered = 0)', () => {
    const { container } = render(
      <TrendsCostDistributionCard
        data={makeData({ covered_session_count: 0, total_session_count: 5 })}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the ⓘ unit caveat only when a model filter is active', () => {
    const { rerender } = render(<TrendsCostDistributionCard data={makeData()} />);
    expect(screen.queryByRole('note')).not.toBeInTheDocument();

    rerender(<TrendsCostDistributionCard data={makeData()} modelFilterActive />);
    expect(screen.getByRole('note', { name: /session, model/i })).toBeInTheDocument();
  });
});
