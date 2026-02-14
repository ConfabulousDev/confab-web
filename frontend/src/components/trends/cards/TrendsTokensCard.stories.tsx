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
