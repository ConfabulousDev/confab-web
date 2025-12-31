import type { Meta, StoryObj } from '@storybook/react-vite';
import { AgentsCard } from './AgentsCard';

const meta: Meta<typeof AgentsCard> = {
  title: 'Session/Cards/AgentsCard',
  component: AgentsCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '300px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AgentsCard>;

export const Default: Story = {
  args: {
    data: {
      total_invocations: 8,
      agent_stats: {
        Explore: { success: 4, errors: 0 },
        Plan: { success: 2, errors: 0 },
        'general-purpose': { success: 2, errors: 0 },
      },
    },
    loading: false,
  },
};

export const WithErrors: Story = {
  args: {
    data: {
      total_invocations: 12,
      agent_stats: {
        Explore: { success: 6, errors: 1 },
        'general-purpose': { success: 3, errors: 2 },
      },
    },
    loading: false,
  },
};

export const SingleAgent: Story = {
  args: {
    data: {
      total_invocations: 5,
      agent_stats: {
        Explore: { success: 5, errors: 0 },
      },
    },
    loading: false,
  },
};

export const ManyAgents: Story = {
  args: {
    data: {
      total_invocations: 25,
      agent_stats: {
        Explore: { success: 10, errors: 0 },
        'general-purpose': { success: 6, errors: 1 },
        Plan: { success: 3, errors: 0 },
        'code-reviewer': { success: 2, errors: 0 },
        'statusline-setup': { success: 1, errors: 1 },
        'claude-code-guide': { success: 1, errors: 0 },
      },
    },
    loading: false,
  },
};

export const AllErrors: Story = {
  args: {
    data: {
      total_invocations: 4,
      agent_stats: {
        'general-purpose': { success: 0, errors: 3 },
        Explore: { success: 0, errors: 1 },
      },
    },
    loading: false,
  },
};

export const NoAgents: Story = {
  args: {
    data: {
      total_invocations: 0,
      agent_stats: {},
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'When no agents are used, the card is not rendered (returns null)',
      },
    },
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
  },
};
