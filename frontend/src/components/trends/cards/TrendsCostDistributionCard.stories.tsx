import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsCostDistributionCard } from './TrendsCostDistributionCard';
import type { TrendsCostDistributionBucket } from '@/schemas/api';

const meta: Meta<typeof TrendsCostDistributionCard> = {
  title: 'Trends/Cards/TrendsCostDistributionCard',
  component: TrendsCostDistributionCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    // The card spans 2 grid columns in situ; widen the isolated frame so the
    // wider histogram and slanted x-axis labels render at a representative size.
    (Story) => (
      <div style={{ width: '760px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsCostDistributionCard>;

// Build buckets from an explicit list of dynamic log10 band labels.
function bucketsFor(
  labels: string[],
  counts: number[],
  totals: string[],
): TrendsCostDistributionBucket[] {
  return labels.map((label, i) => ({
    label,
    lo: i,
    hi: i + 1,
    session_count: counts[i] ?? 0,
    total_usd: totals[i] ?? '0',
  }));
}

// Priced decades only — sub-cent sessions are excluded, so there is no '< $0.01' band.
const BANDS_TO_100 = ['$0.01 – $0.10', '$0.10 – $1', '$1 – $10', '$10 – $100'];

function buckets(counts: number[], totals: string[]): TrendsCostDistributionBucket[] {
  return bucketsFor(BANDS_TO_100, counts, totals);
}

// Typical long-tail shape: many cheap sessions, a thin tail of expensive ones.
export const Default: Story = {
  args: {
    data: {
      buckets: buckets([21, 34, 12, 3], ['1.20', '14.80', '52.30', '88.00']),
      stats: { p50: '0.32', p90: '4.10', p99: '21.40', avg: '3.85' },
      covered_session_count: 70,
      total_session_count: 91,
      timed_out: false,
    },
  },
};

// Single populated band (e.g. a short range where everything cost about the same).
export const SingleBand: Story = {
  args: {
    data: {
      buckets: buckets([0, 6, 0, 0], ['0', '3.90', '0', '0']),
      stats: { p50: '0.62', p90: '0.81', p99: '0.95', avg: '0.65' },
      covered_session_count: 6,
      total_session_count: 6,
      timed_out: false,
    },
  },
};

// Model filter active: bars count (session, model) pairs — the ⓘ caveat appears.
export const ModelFilterActive: Story = {
  args: {
    modelFilterActive: true,
    data: {
      buckets: buckets([9, 11, 5, 1], ['0.55', '4.30', '18.90', '12.50']),
      stats: { p50: '0.21', p90: '2.80', p99: '12.50', avg: '2.10' },
      covered_session_count: 30,
      total_session_count: 30,
      timed_out: false,
    },
  },
};

// Wide range: a few very expensive sessions extend the bands up into the
// millions — decades grow dynamically and totals abbreviate ($1K, $1M).
export const WideRange: Story = {
  args: {
    data: {
      buckets: bucketsFor(
        [
          '$0.01 – $0.10',
          '$0.10 – $1',
          '$1 – $10',
          '$10 – $100',
          '$100 – $1K',
          '$1K – $10K',
          '$10K – $100K',
          '$100K – $1M',
          '$1M – $10M',
        ],
        [18, 27, 14, 5, 2, 0, 0, 0, 1],
        ['1.10', '12.40', '58.00', '210.00', '900.00', '0', '0', '0', '2100000.00'],
      ),
      stats: { p50: '0.36', p90: '7.80', p99: '420.00', avg: '52.00' },
      covered_session_count: 67,
      total_session_count: 80,
      timed_out: false,
    },
  },
};

// Degraded state: the aggregation timed out, so the card shows a narrow-scope notice.
export const TimedOut: Story = {
  args: {
    data: {
      buckets: [],
      stats: null,
      covered_session_count: 0,
      total_session_count: 0,
      timed_out: true,
    },
  },
};
