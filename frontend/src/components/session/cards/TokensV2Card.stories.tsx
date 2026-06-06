import type { Meta, StoryObj } from '@storybook/react-vite';
import { TokensV2Card, type TokensV2CardData } from './TokensV2Card';

const meta: Meta<typeof TokensV2Card> = {
  title: 'Session/Cards/TokensV2Card',
  component: TokensV2Card,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: '280px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TokensV2Card>;

const anthropicModel = {
  input: 100000,
  output: 30000,
  cache_read: 20000,
  cache_write: 5000,
  reasoning: 10000,
  cost_usd: '0.95',
};

const openaiModel = {
  input: 50000,
  output: 20000,
  cache_read: 10000,
  cache_write: 0,
  reasoning: 0,
  cost_usd: '0.28',
};

const multiProvider: TokensV2CardData = {
  total_cost_usd: '1.23',
  total_input: 150000,
  total_output: 50000,
  by_provider: {
    anthropic: { cost_usd: '0.95', models: { 'claude-sonnet-4-20250514': anthropicModel } },
    openai: { cost_usd: '0.28', models: { 'gpt-4o': openaiModel } },
  },
};

export const Default: Story = {
  args: { loading: false, data: multiProvider },
};

export const SingleProvider: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '0.95',
      total_input: 100000,
      total_output: 30000,
      by_provider: {
        anthropic: { cost_usd: '0.95', models: { 'claude-sonnet-4-20250514': anthropicModel } },
      },
    },
  },
};

export const MultiProvider: Story = {
  args: { loading: false, data: multiProvider },
};

export const ZeroCost: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '0.00',
      total_input: 0,
      total_output: 0,
      by_provider: {
        anthropic: {
          cost_usd: '0.00',
          models: {
            'claude-haiku-4-5': { input: 0, output: 0, cache_read: 0, cache_write: 0, reasoning: 0, cost_usd: '0.00' },
          },
        },
      },
    },
  },
};

export const HighUsage: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '42.67',
      total_input: 8_400_000,
      total_output: 2_100_000,
      by_provider: {
        anthropic: {
          cost_usd: '31.20',
          models: {
            'claude-opus-4-8': { input: 5_000_000, output: 1_400_000, cache_read: 12_000_000, cache_write: 800_000, reasoning: 600_000, cost_usd: '31.20' },
          },
        },
        google: {
          cost_usd: '11.47',
          models: {
            'gemini-2.5-pro': { input: 3_400_000, output: 700_000, cache_read: 1_200_000, cache_write: 0, reasoning: 0, cost_usd: '11.47' },
          },
        },
      },
    },
  },
};

export const Loading: Story = {
  args: { loading: true, data: null },
};
