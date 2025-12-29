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
    },
    loading: false,
  },
};

export const LowUsage: Story = {
  args: {
    data: {
      input: 1500,
      output: 800,
      cache_creation: 500,
      cache_read: 2000,
    },
    loading: false,
  },
};

export const HighUsage: Story = {
  args: {
    data: {
      input: 2500000,
      output: 1200000,
      cache_creation: 500000,
      cache_read: 8500000,
    },
    loading: false,
  },
};

export const NoCaching: Story = {
  args: {
    data: {
      input: 50000,
      output: 25000,
      cache_creation: 0,
      cache_read: 0,
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
