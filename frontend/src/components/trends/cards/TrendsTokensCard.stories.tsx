import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsTokensCard } from './TrendsTokensCard';
import type { TrendsTokensPerProvider } from '@/schemas/api';

const meta: Meta<typeof TrendsTokensCard> = {
  title: 'Trends/Cards/TrendsTokensCard',
  component: TrendsTokensCard,
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
type Story = StoryObj<typeof TrendsTokensCard>;

// Helper for single-provider story fixtures: the per_provider entry exactly
// mirrors the top-level totals. Keeps single-provider stories from duplicating
// the same five numbers twice.
function singleProvider(
  providerId: string,
  totals: TrendsTokensPerProvider,
): Record<string, TrendsTokensPerProvider> {
  return { [providerId]: totals };
}

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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 5000000,
        total_output_tokens: 2000000,
        total_cache_creation_tokens: 100000,
        total_cache_read_tokens: 500000,
        total_cost_usd: '125.50',
      }),
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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 50000,
        total_output_tokens: 25000,
        total_cache_creation_tokens: 5000,
        total_cache_read_tokens: 10000,
        total_cost_usd: '2.50',
      }),
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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 50000000,
        total_output_tokens: 20000000,
        total_cache_creation_tokens: 1000000,
        total_cache_read_tokens: 5000000,
        total_cost_usd: '1250.00',
      }),
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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 100000,
        total_output_tokens: 50000,
        total_cache_creation_tokens: 10000,
        total_cache_read_tokens: 20000,
        total_cost_usd: '5.00',
      }),
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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 100000,
        total_output_tokens: 50000,
        total_cache_creation_tokens: 10000,
        total_cache_read_tokens: 20000,
        total_cost_usd: '0.00',
      }),
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};

// CF-435: single-provider filter — renders the existing StatRow layout.
export const SingleProvider: Story = {
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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 1_000_000,
        total_output_tokens: 400_000,
        total_cache_creation_tokens: 50_000,
        total_cache_read_tokens: 200_000,
        total_cost_usd: '25.00',
      }),
    },
  },
};

// CF-436: Codex-only filtered window has no cache writes. The cache row
// collapses to a single "Cache Read" line under the existing single-provider
// layout.
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
      per_provider: singleProvider('codex', {
        total_input_tokens: 800_000,
        total_output_tokens: 150_000,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 120_000,
        total_cost_usd: '4.25',
      }),
    },
  },
};

// CF-436: window where no caching happened at all. The entire cache row is
// hidden in the single-provider layout.
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
      per_provider: singleProvider('claude-code', {
        total_input_tokens: 200_000,
        total_output_tokens: 80_000,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 0,
        total_cost_usd: '3.10',
      }),
    },
  },
};

// CF-435: multi-provider filtered set — renders the per-provider table in
// place of the single-series StatRow stack. Codex has no cache writes by
// design (em-dash); Total row dashes out the four token columns and rolls up
// only Cost.
export const MultiProviderTable: Story = {
  args: {
    data: {
      total_input_tokens: 5_800_000,
      total_output_tokens: 2_150_000,
      total_cache_creation_tokens: 100_000,
      total_cache_read_tokens: 620_000,
      total_cost_usd: '129.25',
      daily_costs: [
        { date: '2024-01-08', cost_usd: '20.00' },
        { date: '2024-01-09', cost_usd: '24.50' },
        { date: '2024-01-10', cost_usd: '17.25' },
        { date: '2024-01-11', cost_usd: '21.50' },
        { date: '2024-01-12', cost_usd: '46.00' },
      ],
      per_provider: {
        'claude-code': {
          total_input_tokens: 5_000_000,
          total_output_tokens: 2_000_000,
          total_cache_creation_tokens: 100_000,
          total_cache_read_tokens: 500_000,
          total_cost_usd: '125.00',
        },
        codex: {
          total_input_tokens: 800_000,
          total_output_tokens: 150_000,
          total_cache_creation_tokens: 0,
          total_cache_read_tokens: 120_000,
          total_cost_usd: '4.25',
        },
      },
    },
  },
};
