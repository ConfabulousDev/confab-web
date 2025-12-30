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
      // Message counts
      total_messages: 100,
      user_messages: 50,
      assistant_messages: 50,
      // Message type breakdown
      human_prompts: 15,
      tool_results: 35,
      text_responses: 15,
      tool_calls: 30,
      thinking_blocks: 5,
      // Metadata
      duration_ms: 3600000, // 1 hour
      models_used: ['claude-sonnet-4-20250514'],
      compaction_auto: 0,
      compaction_manual: 0,
      compaction_avg_time_ms: null,
    },
    loading: false,
  },
};

export const ShortSession: Story = {
  args: {
    data: {
      total_messages: 10,
      user_messages: 5,
      assistant_messages: 5,
      human_prompts: 3,
      tool_results: 2,
      text_responses: 3,
      tool_calls: 2,
      thinking_blocks: 0,
      duration_ms: 180000, // 3 minutes
      models_used: ['claude-sonnet-4-20250514'],
      compaction_auto: 0,
      compaction_manual: 0,
      compaction_avg_time_ms: null,
    },
    loading: false,
  },
};

export const LongSession: Story = {
  args: {
    data: {
      total_messages: 850,
      user_messages: 400,
      assistant_messages: 450,
      human_prompts: 85,
      tool_results: 315,
      text_responses: 85,
      tool_calls: 280,
      thinking_blocks: 85,
      duration_ms: 14400000, // 4 hours
      models_used: ['claude-opus-4-5-20251101'],
      compaction_auto: 3,
      compaction_manual: 1,
      compaction_avg_time_ms: 4500,
    },
    loading: false,
  },
};

export const MultipleModels: Story = {
  args: {
    data: {
      total_messages: 200,
      user_messages: 95,
      assistant_messages: 105,
      human_prompts: 25,
      tool_results: 70,
      text_responses: 25,
      tool_calls: 60,
      thinking_blocks: 20,
      duration_ms: 5400000, // 1.5 hours
      models_used: ['claude-sonnet-4-20250514', 'claude-opus-4-5-20251101'],
      compaction_auto: 1,
      compaction_manual: 0,
      compaction_avg_time_ms: 3200,
    },
    loading: false,
  },
};

export const WithCompaction: Story = {
  args: {
    data: {
      total_messages: 500,
      user_messages: 240,
      assistant_messages: 260,
      human_prompts: 50,
      tool_results: 190,
      text_responses: 50,
      tool_calls: 160,
      thinking_blocks: 50,
      duration_ms: 7200000, // 2 hours
      models_used: ['claude-sonnet-4-20250514'],
      compaction_auto: 5,
      compaction_manual: 2,
      compaction_avg_time_ms: 6500,
    },
    loading: false,
  },
};

export const NoDuration: Story = {
  args: {
    data: {
      total_messages: 20,
      user_messages: 10,
      assistant_messages: 10,
      human_prompts: 5,
      tool_results: 5,
      text_responses: 5,
      tool_calls: 4,
      thinking_blocks: 1,
      duration_ms: null,
      models_used: ['claude-sonnet-4-20250514'],
      compaction_auto: 0,
      compaction_manual: 0,
      compaction_avg_time_ms: null,
    },
    loading: false,
  },
};

export const NoModels: Story = {
  args: {
    data: {
      total_messages: 40,
      user_messages: 20,
      assistant_messages: 20,
      human_prompts: 10,
      tool_results: 10,
      text_responses: 10,
      tool_calls: 8,
      thinking_blocks: 2,
      duration_ms: 1800000,
      models_used: [],
      compaction_auto: 0,
      compaction_manual: 0,
      compaction_avg_time_ms: null,
    },
    loading: false,
  },
};

export const ToolHeavySession: Story = {
  args: {
    data: {
      // Real-world example similar to CF-164 investigation
      total_messages: 1938,
      user_messages: 635,
      assistant_messages: 1303,
      human_prompts: 49,
      tool_results: 586,
      text_responses: 49,
      tool_calls: 726,
      thinking_blocks: 528,
      duration_ms: 28800000, // 8 hours
      models_used: ['claude-sonnet-4-20250514', 'claude-opus-4-5-20251101'],
      compaction_auto: 8,
      compaction_manual: 3,
      compaction_avg_time_ms: 5200,
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
