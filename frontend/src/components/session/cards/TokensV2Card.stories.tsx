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

// hp73: deep multi-provider × multi-model case — the whole point of the
// redesign is that the three indentation levels (provider → model → detail)
// read clearly. Each provider has two models so the collapsed model headlines
// nest visibly under the uppercase provider header.
export const DeepMultiProviderMultiModel: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '12.40',
      total_input: 3_600_000,
      total_output: 720_000,
      by_provider: {
        anthropic: {
          cost_usd: '9.10',
          models: {
            'opus-4-8': { input: 2_000_000, output: 500_000, cache_read: 1_500_000, cache_write: 120_000, reasoning: 0, cost_usd: '7.20' },
            'opus-4-8 · fast': { input: 400_000, output: 80_000, cache_read: 0, cache_write: 0, reasoning: 24_000, cost_usd: '1.90' },
          },
        },
        openai: {
          cost_usd: '3.30',
          models: {
            'gpt-5': { input: 900_000, output: 120_000, cache_read: 300_000, cache_write: 0, reasoning: 30_000, cost_usd: '2.10' },
            'gpt-5-mini': { input: 300_000, output: 20_000, cache_read: 100_000, cache_write: 0, reasoning: 0, cost_usd: '1.20' },
          },
        },
      },
    },
  },
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

// 7eje/mp4e: Claude/Codex use the canonical agent id as the single by_provider
// key and getModelFamily() families as model keys (fast under "<family> · fast").
// Single provider → no provider section header; model labels are formatted.
export const ClaudeSingleModel: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '4.20',
      total_input: 1_000_000,
      total_output: 300_000,
      by_provider: {
        'claude-code': {
          cost_usd: '4.20',
          models: {
            'opus-4-8': { input: 1_000_000, output: 300_000, cache_read: 800_000, cache_write: 50_000, reasoning: 0, cost_usd: '4.20' },
          },
        },
      },
    },
  },
};

export const ClaudeWithFastVariant: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '4.90',
      total_input: 1_200_000,
      total_output: 340_000,
      by_provider: {
        'claude-code': {
          cost_usd: '4.90',
          models: {
            'opus-4-8': { input: 1_000_000, output: 300_000, cache_read: 800_000, cache_write: 0, reasoning: 0, cost_usd: '3.50' },
            'opus-4-8 · fast': { input: 200_000, output: 40_000, cache_read: 0, cache_write: 0, reasoning: 0, cost_usd: '1.40' },
          },
        },
      },
    },
  },
};

export const CodexWithReasoning: Story = {
  args: {
    loading: false,
    data: {
      total_cost_usd: '2.15',
      total_input: 600_000,
      total_output: 90_000,
      by_provider: {
        codex: {
          cost_usd: '2.15',
          models: {
            'gpt-5': { input: 600_000, output: 90_000, cache_read: 400_000, cache_write: 0, reasoning: 18_000, cost_usd: '2.15' },
          },
        },
      },
    },
  },
};

export const Loading: Story = {
  args: { loading: true, data: null },
};
