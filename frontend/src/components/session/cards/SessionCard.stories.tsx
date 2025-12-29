import type { Meta, StoryObj } from '@storybook/react-vite';
import { SessionCard } from './SessionCard';

const meta: Meta<typeof SessionCard> = {
  title: 'Session/Cards/SessionCard',
  component: SessionCard,
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
type Story = StoryObj<typeof SessionCard>;

export const Default: Story = {
  args: {
    data: {
      user_turns: 15,
      assistant_turns: 15,
      duration_ms: 3600000, // 1 hour
      models_used: ['claude-sonnet-4-20250514'],
    },
    loading: false,
  },
};

export const ShortSession: Story = {
  args: {
    data: {
      user_turns: 3,
      assistant_turns: 3,
      duration_ms: 180000, // 3 minutes
      models_used: ['claude-sonnet-4-20250514'],
    },
    loading: false,
  },
};

export const LongSession: Story = {
  args: {
    data: {
      user_turns: 85,
      assistant_turns: 85,
      duration_ms: 14400000, // 4 hours
      models_used: ['claude-opus-4-5-20251101'],
    },
    loading: false,
  },
};

export const MultipleModels: Story = {
  args: {
    data: {
      user_turns: 25,
      assistant_turns: 25,
      duration_ms: 5400000, // 1.5 hours
      models_used: ['claude-sonnet-4-20250514', 'claude-opus-4-5-20251101'],
    },
    loading: false,
  },
};

export const NoDuration: Story = {
  args: {
    data: {
      user_turns: 5,
      assistant_turns: 5,
      duration_ms: null,
      models_used: ['claude-sonnet-4-20250514'],
    },
    loading: false,
  },
};

export const NoModels: Story = {
  args: {
    data: {
      user_turns: 10,
      assistant_turns: 10,
      duration_ms: 1800000,
      models_used: [],
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
