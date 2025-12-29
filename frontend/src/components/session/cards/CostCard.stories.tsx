import type { Meta, StoryObj } from '@storybook/react-vite';
import { CostCard } from './CostCard';

const meta: Meta<typeof CostCard> = {
  title: 'Session/Cards/CostCard',
  component: CostCard,
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
type Story = StoryObj<typeof CostCard>;

export const Default: Story = {
  args: {
    data: {
      estimated_usd: '2.45',
    },
    loading: false,
  },
};

export const LowCost: Story = {
  args: {
    data: {
      estimated_usd: '0.03',
    },
    loading: false,
  },
};

export const HighCost: Story = {
  args: {
    data: {
      estimated_usd: '15.87',
    },
    loading: false,
  },
};

export const ZeroCost: Story = {
  args: {
    data: {
      estimated_usd: '0.00',
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
