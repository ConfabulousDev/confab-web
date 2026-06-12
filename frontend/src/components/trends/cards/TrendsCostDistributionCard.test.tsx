import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import {
  TrendsCostDistributionCard,
  CostDistributionTooltip,
} from './TrendsCostDistributionCard';
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

// A representative Recharts tooltip payload for one bar (dataKey="session_count").
// Recharts nests the full chart row under payload[0].payload.
function payloadFor(bucket: TrendsCostDistributionBucket) {
  return [
    {
      name: 'session_count',
      value: bucket.session_count,
      payload: {
        label: bucket.label,
        session_count: bucket.session_count,
        total_usd: bucket.total_usd,
      },
    },
  ];
}

describe('TrendsCostDistributionCard', () => {
  it('labels the histogram so bar height reads as session count', () => {
    render(<TrendsCostDistributionCard data={makeData()} />);
    expect(screen.getByText('Sessions per cost band')).toBeInTheDocument();
  });

  it('labels the histogram in session-model-pair terms when a model filter is active', () => {
    render(<TrendsCostDistributionCard data={makeData()} modelFilterActive />);
    expect(screen.getByText('Session-model pairs per cost band')).toBeInTheDocument();
  });

  it('does not render per-bar total-$ labels up front (they moved to hover)', () => {
    render(<TrendsCostDistributionCard data={makeData()} />);
    // These band totals are distinct from any percentile value, so their
    // absence proves the per-bar labels are gone (not hidden by a collision).
    expect(screen.queryByText('$50.00')).not.toBeInTheDocument();
    expect(screen.queryByText('$1.50')).not.toBeInTheDocument();
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
    // The chart still renders (its label is present).
    expect(screen.getByText('Sessions per cost band')).toBeInTheDocument();
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
    expect(screen.queryByText('Sessions per cost band')).not.toBeInTheDocument();
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

describe('CostDistributionTooltip', () => {
  const bucket: TrendsCostDistributionBucket = {
    label: '$0.10 – $1',
    lo: 2,
    hi: 3,
    session_count: 34,
    total_usd: '14.80',
  };

  it('renders nothing when inactive', () => {
    const { container } = render(
      <CostDistributionTooltip active={false} payload={payloadFor(bucket)} unit="sessions" />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when the payload is empty', () => {
    const { container } = render(
      <CostDistributionTooltip active payload={[]} unit="sessions" />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the band label, the session count, and the band total', () => {
    render(<CostDistributionTooltip active payload={payloadFor(bucket)} unit="sessions" />);
    expect(screen.getByText('$0.10 – $1')).toBeInTheDocument();
    expect(screen.getByText(/34 sessions/)).toBeInTheDocument();
    expect(screen.getByText(/\$14\.80 total/)).toBeInTheDocument();
  });

  it('uses the session-model-pairs unit when a model filter is active', () => {
    render(
      <CostDistributionTooltip
        active
        payload={payloadFor(bucket)}
        unit="session-model pairs"
      />,
    );
    expect(screen.getByText(/34 session-model pairs/)).toBeInTheDocument();
  });

  it('abbreviates large band totals compactly ($M)', () => {
    render(
      <CostDistributionTooltip
        active
        payload={payloadFor({ ...bucket, total_usd: '2100000.00' })}
        unit="sessions"
      />,
    );
    expect(screen.getByText(/\$2\.1M total/)).toBeInTheDocument();
  });

  it('floors a tiny non-zero band total to <$0.01', () => {
    render(
      <CostDistributionTooltip
        active
        payload={payloadFor({ ...bucket, total_usd: '0.005' })}
        unit="sessions"
      />,
    );
    expect(screen.getByText(/<\$0\.01 total/)).toBeInTheDocument();
  });
});
