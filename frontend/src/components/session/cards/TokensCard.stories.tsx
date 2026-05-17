import type { Meta, StoryObj } from '@storybook/react-vite';
import { TokensCard } from './TokensCard';

const meta: Meta<typeof TokensCard> = {
  title: 'Session/Cards/TokensCard',
  component: TokensCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '280px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TokensCard>;

export const Default: Story = {
  args: {
    data: {
      input: 125000,
      output: 45000,
      cache_creation: 80000,
      cache_read: 320000,
      estimated_usd: '1.85',
    },
    loading: false,
    provider: 'claude-code',
  },
};

export const WithFastMode: Story = {
  args: {
    data: {
      input: 250000,
      output: 90000,
      cache_creation: 100000,
      cache_read: 500000,
      estimated_usd: '12.40',
      fast_turns: 15,
      fast_cost_usd: '9.60',
    },
    loading: false,
    provider: 'claude-code',
  },
};

export const AllFastMode: Story = {
  args: {
    data: {
      input: 125000,
      output: 45000,
      cache_creation: 80000,
      cache_read: 320000,
      estimated_usd: '11.10',
      fast_turns: 25,
      fast_cost_usd: '11.10',
    },
    loading: false,
    provider: 'claude-code',
  },
};

export const LowUsage: Story = {
  args: {
    data: {
      input: 1500,
      output: 800,
      cache_creation: 500,
      cache_read: 2000,
      estimated_usd: '0.02',
    },
    loading: false,
    provider: 'claude-code',
  },
};

export const HighUsage: Story = {
  args: {
    data: {
      input: 2500000,
      output: 1200000,
      cache_creation: 500000,
      cache_read: 8500000,
      estimated_usd: '45.50',
    },
    loading: false,
    provider: 'claude-code',
  },
};

// CF-436: Claude session with no prompt caching. The "Cache created" row
// is hidden (value-based gate); "Cache read" still shows 0.
export const NoCaching: Story = {
  args: {
    data: {
      input: 50000,
      output: 25000,
      cache_creation: 0,
      cache_read: 0,
      estimated_usd: '0.75',
    },
    loading: false,
    provider: 'claude-code',
  },
};

export const ZeroCost: Story = {
  args: {
    data: {
      input: 100,
      output: 50,
      cache_creation: 0,
      cache_read: 100,
      estimated_usd: '0.00',
    },
    loading: false,
    provider: 'claude-code',
  },
};

// CF-436: Codex sessions always have cache_creation === 0 (OpenAI doesn't
// bill cache writes) and never expose fast_turns. The card renders 4 rows:
// Estimated cost, Input, Output, Cache read.
export const Codex: Story = {
  args: {
    data: {
      input: 80_000,
      output: 5_000,
      cache_creation: 0,
      cache_read: 12_000,
      estimated_usd: '0.42',
    },
    loading: false,
    provider: 'codex',
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
    provider: 'claude-code',
  },
};
