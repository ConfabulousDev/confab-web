import type { Meta, StoryObj } from '@storybook/react-vite';
import { CompactionCard } from './CompactionCard';

const meta: Meta<typeof CompactionCard> = {
  title: 'Session/Cards/CompactionCard',
  component: CompactionCard,
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
type Story = StoryObj<typeof CompactionCard>;

export const Default: Story = {
  args: {
    data: {
      auto: 3,
      manual: 1,
      avg_time_ms: 2450,
    },
    loading: false,
  },
};

export const AutoOnly: Story = {
  args: {
    data: {
      auto: 5,
      manual: 0,
      avg_time_ms: 1850,
    },
    loading: false,
  },
};

export const ManualOnly: Story = {
  args: {
    data: {
      auto: 0,
      manual: 2,
      avg_time_ms: null,
    },
    loading: false,
  },
};

export const NoCompactions: Story = {
  args: {
    data: {
      auto: 0,
      manual: 0,
      avg_time_ms: null,
    },
    loading: false,
  },
};

export const SlowCompactions: Story = {
  args: {
    data: {
      auto: 2,
      manual: 0,
      avg_time_ms: 8500,
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
