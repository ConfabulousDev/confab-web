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
