import type { Meta, StoryObj } from '@storybook/react-vite';
import { FastModeCard } from './FastModeCard';

const meta: Meta<typeof FastModeCard> = {
  title: 'Session/Cards/FastModeCard',
  component: FastModeCard,
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
type Story = StoryObj<typeof FastModeCard>;

export const Default: Story = {
  args: {
    data: {
      fast_turns: 12,
      standard_turns: 38,
      fast_cost_usd: '2.4500',
      standard_cost_usd: '1.2300',
    },
    loading: false,
  },
};

export const AllFast: Story = {
  args: {
    data: {
      fast_turns: 25,
      standard_turns: 0,
      fast_cost_usd: '5.1200',
      standard_cost_usd: '0',
    },
    loading: false,
  },
};

export const MostlyStandard: Story = {
  args: {
    data: {
      fast_turns: 2,
      standard_turns: 48,
      fast_cost_usd: '0.3200',
      standard_cost_usd: '1.8700',
    },
    loading: false,
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
  },
};

export const Error: Story = {
  args: {
    data: undefined,
    loading: false,
    error: 'Failed to compute fast mode analytics',
  },
};

export const Empty: Story = {
  args: {
    data: {
      fast_turns: 0,
      standard_turns: 50,
      fast_cost_usd: '0',
      standard_cost_usd: '2.5000',
    },
    loading: false,
  },
};
