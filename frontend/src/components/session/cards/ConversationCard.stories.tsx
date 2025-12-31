import type { Meta, StoryObj } from '@storybook/react-vite';
import { ConversationCard } from './ConversationCard';

const meta: Meta<typeof ConversationCard> = {
  title: 'Session/Cards/ConversationCard',
  component: ConversationCard,
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
type Story = StoryObj<typeof ConversationCard>;

export const Default: Story = {
  args: {
    data: {
      user_turns: 15,
      assistant_turns: 15,
      avg_assistant_turn_ms: 45000, // 45 seconds
      avg_user_thinking_ms: 120000, // 2 minutes
      total_assistant_duration_ms: 675000, // 11m 15s (15 * 45s)
      total_user_duration_ms: 1800000, // 30 minutes (15 * 2m)
      assistant_utilization: 27.3, // 675000 / (675000 + 1800000) * 100
    },
    loading: false,
  },
};

export const QuickResponses: Story = {
  args: {
    data: {
      user_turns: 25,
      assistant_turns: 25,
      avg_assistant_turn_ms: 8000, // 8 seconds
      avg_user_thinking_ms: 15000, // 15 seconds
      total_assistant_duration_ms: 200000, // 3m 20s (25 * 8s)
      total_user_duration_ms: 375000, // 6m 15s (25 * 15s)
      assistant_utilization: 34.8, // 200000 / (200000 + 375000) * 100
    },
    loading: false,
  },
};

export const LongSession: Story = {
  args: {
    data: {
      user_turns: 85,
      assistant_turns: 85,
      avg_assistant_turn_ms: 180000, // 3 minutes
      avg_user_thinking_ms: 600000, // 10 minutes
      total_assistant_duration_ms: 15300000, // 4h 15m (85 * 3m)
      total_user_duration_ms: 51000000, // 14h 10m (85 * 10m)
      assistant_utilization: 23.1, // 15300000 / (15300000 + 51000000) * 100
    },
    loading: false,
  },
};

export const VeryLongTurns: Story = {
  args: {
    data: {
      user_turns: 10,
      assistant_turns: 10,
      avg_assistant_turn_ms: 3600000, // 1 hour
      avg_user_thinking_ms: 1800000, // 30 minutes
      total_assistant_duration_ms: 36000000, // 10 hours (10 * 1h)
      total_user_duration_ms: 18000000, // 5 hours (10 * 30m)
      assistant_utilization: 66.7, // 36000000 / (36000000 + 18000000) * 100
    },
    loading: false,
  },
};

export const ShortSession: Story = {
  args: {
    data: {
      user_turns: 3,
      assistant_turns: 3,
      avg_assistant_turn_ms: 5000, // 5 seconds
      avg_user_thinking_ms: 10000, // 10 seconds
      total_assistant_duration_ms: 15000, // 15s (3 * 5s)
      total_user_duration_ms: 30000, // 30s (3 * 10s)
      assistant_utilization: 33.3, // 15000 / (15000 + 30000) * 100
    },
    loading: false,
  },
};

export const NoTimingData: Story = {
  args: {
    data: {
      user_turns: 5,
      assistant_turns: 5,
      avg_assistant_turn_ms: null,
      avg_user_thinking_ms: null,
      total_assistant_duration_ms: null,
      total_user_duration_ms: null,
      assistant_utilization: null,
    },
    loading: false,
  },
};

export const OnlyAssistantTiming: Story = {
  args: {
    data: {
      user_turns: 8,
      assistant_turns: 8,
      avg_assistant_turn_ms: 30000, // 30 seconds
      avg_user_thinking_ms: null,
      total_assistant_duration_ms: 240000, // 4 minutes (8 * 30s)
      total_user_duration_ms: null, // No user timing data
      assistant_utilization: null, // Can't compute without both
    },
    loading: false,
  },
};

export const SubSecondTiming: Story = {
  args: {
    data: {
      user_turns: 50,
      assistant_turns: 50,
      avg_assistant_turn_ms: 500, // 500ms
      avg_user_thinking_ms: 250, // 250ms
      total_assistant_duration_ms: 25000, // 25s (50 * 500ms)
      total_user_duration_ms: 12500, // 12.5s (50 * 250ms)
      assistant_utilization: 66.7, // 25000 / (25000 + 12500) * 100
    },
    loading: false,
  },
};

export const HighUtilization: Story = {
  args: {
    data: {
      user_turns: 20,
      assistant_turns: 20,
      avg_assistant_turn_ms: 120000, // 2 minutes
      avg_user_thinking_ms: 5000, // 5 seconds
      total_assistant_duration_ms: 2400000, // 40 minutes (20 * 2m)
      total_user_duration_ms: 100000, // 1m 40s (20 * 5s)
      assistant_utilization: 96.0, // 2400000 / (2400000 + 100000) * 100
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
