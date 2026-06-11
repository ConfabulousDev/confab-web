import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsCostByModelCard } from './TrendsCostByModelCard';
import type { TrendsCostByModelRow } from '@/schemas/api';

const meta: Meta<typeof TrendsCostByModelCard> = {
  title: 'Trends/Cards/TrendsCostByModelCard',
  component: TrendsCostByModelCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '480px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsCostByModelCard>;

function row(overrides: Partial<TrendsCostByModelRow> = {}): TrendsCostByModelRow {
  return {
    model: 'opus-4-5',
    provider: 'claude-code',
    cost_usd: '1.00',
    pct_of_total: 25,
    input: 120_000,
    output: 48_000,
    cache_read: 2_400_000,
    cache_write: 180_000,
    session_count: 12,
    ...overrides,
  };
}

// Multi-provider breakdown: the same family under two providers stays as
// distinct rows; a "· fast" variant and the Unknown (unpriced) row appear.
export const Default: Story = {
  args: {
    data: {
      rows: [
        row({ provider: 'codex', model: 'gpt-5', cost_usd: '18.40', pct_of_total: 52.1, session_count: 30 }),
        row({ provider: 'claude-code', model: 'opus-4-5', cost_usd: '9.80', pct_of_total: 27.8, session_count: 21 }),
        row({ provider: 'claude-code', model: 'opus-4-5 · fast', cost_usd: '4.20', pct_of_total: 11.9, session_count: 9 }),
        row({ provider: 'opencode', model: 'opus-4-5', cost_usd: '2.10', pct_of_total: 5.9, session_count: 4 }),
        row({ provider: 'opencode', model: 'gpt-5', cost_usd: '0.80', pct_of_total: 2.3, session_count: 2 }),
        row({ provider: 'claude-code', model: '', cost_usd: '0.00', pct_of_total: 0, session_count: 3, cache_read: 0, cache_write: 0 }),
      ],
      covered_session_count: 48,
      total_session_count: 63,
      timed_out: false,
    },
  },
};

export const SingleModel: Story = {
  args: {
    data: {
      rows: [row({ provider: 'claude-code', model: 'opus-4-8', cost_usd: '6.50', pct_of_total: 100, session_count: 14 })],
      covered_session_count: 14,
      total_session_count: 14,
      timed_out: false,
    },
  },
};

// Degraded state: the aggregation timed out, so the card shows a narrow-scope
// notice rather than a misleading empty breakdown.
export const TimedOut: Story = {
  args: {
    data: {
      rows: [],
      covered_session_count: 0,
      total_session_count: 0,
      timed_out: true,
    },
  },
};
