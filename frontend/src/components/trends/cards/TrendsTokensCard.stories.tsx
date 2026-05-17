import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsTokensCard } from './TrendsTokensCard';

const meta: Meta<typeof TrendsTokensCard> = {
  title: 'Trends/Cards/TrendsTokensCard',
  component: TrendsTokensCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '400px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsTokensCard>;

export const Default: Story = {
  args: {
    data: {
      total_input_tokens: 5000000,
      total_output_tokens: 2000000,
      total_cache_creation_tokens: 100000,
      total_cache_read_tokens: 500000,
      total_cost_usd: '125.50',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '15.20' },
        { date: '2024-01-09', cost_usd: '18.50' },
        { date: '2024-01-10', cost_usd: '12.80' },
        { date: '2024-01-11', cost_usd: '22.00' },
        { date: '2024-01-12', cost_usd: '19.00' },
        { date: '2024-01-13', cost_usd: '25.00' },
        { date: '2024-01-14', cost_usd: '13.00' },
      ],
    },
  },
};

export const LowUsage: Story = {
  args: {
    data: {
      total_input_tokens: 50000,
      total_output_tokens: 25000,
      total_cache_creation_tokens: 5000,
      total_cache_read_tokens: 10000,
      total_cost_usd: '2.50',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '0.50' },
        { date: '2024-01-09', cost_usd: '1.00' },
        { date: '2024-01-10', cost_usd: '1.00' },
      ],
    },
  },
};

export const HighUsage: Story = {
  args: {
    data: {
      total_input_tokens: 50000000,
      total_output_tokens: 20000000,
      total_cache_creation_tokens: 1000000,
      total_cache_read_tokens: 5000000,
      total_cost_usd: '1250.00',
      daily_costs: [
        { date: '2024-01-01', cost_usd: '45.00' },
        { date: '2024-01-02', cost_usd: '52.00' },
        { date: '2024-01-03', cost_usd: '38.00' },
        { date: '2024-01-04', cost_usd: '61.00' },
        { date: '2024-01-05', cost_usd: '44.00' },
        { date: '2024-01-06', cost_usd: '55.00' },
        { date: '2024-01-07', cost_usd: '48.00' },
      ],
    },
  },
};

export const SingleDay: Story = {
  args: {
    data: {
      total_input_tokens: 100000,
      total_output_tokens: 50000,
      total_cache_creation_tokens: 10000,
      total_cache_read_tokens: 20000,
      total_cost_usd: '5.00',
      daily_costs: [{ date: '2024-01-08', cost_usd: '5.00' }],
    },
  },
};

export const ZeroCost: Story = {
  args: {
    data: {
      total_input_tokens: 100000,
      total_output_tokens: 50000,
      total_cache_creation_tokens: 10000,
      total_cache_read_tokens: 20000,
      total_cost_usd: '0.00',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '0.00' },
        { date: '2024-01-09', cost_usd: '0.00' },
      ],
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};

// CF-424: single-provider data → no caveat.
export const WithoutCaveat: Story = {
  args: {
    data: {
      total_input_tokens: 1_000_000,
      total_output_tokens: 400_000,
      total_cache_creation_tokens: 50_000,
      total_cache_read_tokens: 200_000,
      total_cost_usd: '25.00',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '5.00' },
        { date: '2024-01-09', cost_usd: '6.50' },
        { date: '2024-01-10', cost_usd: '4.25' },
        { date: '2024-01-11', cost_usd: '4.75' },
        { date: '2024-01-12', cost_usd: '4.50' },
      ],
    },
    providersPresent: ['claude-code'],
  },
};

// CF-436: Codex-only filtered window has no cache writes. The cache row
// collapses to a single "Cache Read" line.
export const CodexOnlyNoCacheWrites: Story = {
  args: {
    data: {
      total_input_tokens: 800_000,
      total_output_tokens: 150_000,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 120_000,
      total_cost_usd: '4.25',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '1.10' },
        { date: '2024-01-09', cost_usd: '1.50' },
        { date: '2024-01-10', cost_usd: '0.85' },
        { date: '2024-01-11', cost_usd: '0.80' },
      ],
    },
    providersPresent: ['codex'],
  },
};

// CF-436: window where no caching happened at all. The entire cache row is
// hidden.
export const NoCachingAtAll: Story = {
  args: {
    data: {
      total_input_tokens: 200_000,
      total_output_tokens: 80_000,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 0,
      total_cost_usd: '3.10',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '1.00' },
        { date: '2024-01-09', cost_usd: '2.10' },
      ],
    },
    providersPresent: ['claude-code'],
  },
};

// CF-424: multi-provider data → renders the muted caveat line beneath the
// headline numbers. Tokens across Claude and Codex aren't directly comparable.
export const WithCaveat: Story = {
  args: {
    data: {
      total_input_tokens: 1_000_000,
      total_output_tokens: 400_000,
      total_cache_creation_tokens: 50_000,
      total_cache_read_tokens: 200_000,
      total_cost_usd: '25.00',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '5.00' },
        { date: '2024-01-09', cost_usd: '6.50' },
        { date: '2024-01-10', cost_usd: '4.25' },
        { date: '2024-01-11', cost_usd: '4.75' },
        { date: '2024-01-12', cost_usd: '4.50' },
      ],
    },
    providersPresent: ['claude-code', 'codex'],
  },
};
