import type { Meta, StoryObj } from '@storybook/react-vite';
import { AgentsAndSkillsCard } from './AgentsAndSkillsCard';

const meta: Meta<typeof AgentsAndSkillsCard> = {
  title: 'Session/Cards/AgentsAndSkillsCard',
  component: AgentsAndSkillsCard,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: '380px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AgentsAndSkillsCard>;

export const Default: Story = {
  args: {
    data: {
      agent_invocations: 8,
      skill_invocations: 3,
      agent_stats: {
        Explore: { success: 4, errors: 0 },
        Plan: { success: 2, errors: 0 },
        'code-reviewer': { success: 1, errors: 1 },
      },
      skill_stats: {
        commit: { success: 2, errors: 0 },
        'review-pr': { success: 1, errors: 0 },
      },
    },
    loading: false,
  },
};

export const AgentsOnly: Story = {
  args: {
    data: {
      agent_invocations: 5,
      skill_invocations: 0,
      agent_stats: {
        Explore: { success: 3, errors: 0 },
        Plan: { success: 2, errors: 0 },
      },
      skill_stats: {},
    },
    loading: false,
  },
};

export const SkillsOnly: Story = {
  args: {
    data: {
      agent_invocations: 0,
      skill_invocations: 4,
      agent_stats: {},
      skill_stats: {
        commit: { success: 3, errors: 0 },
        bugfix: { success: 1, errors: 0 },
      },
    },
    loading: false,
  },
};

export const WithErrors: Story = {
  args: {
    data: {
      agent_invocations: 6,
      skill_invocations: 3,
      agent_stats: {
        Explore: { success: 3, errors: 1 },
        Plan: { success: 1, errors: 1 },
      },
      skill_stats: {
        commit: { success: 2, errors: 1 },
      },
    },
    loading: false,
  },
};

export const ManyItems: Story = {
  args: {
    data: {
      agent_invocations: 25,
      skill_invocations: 12,
      agent_stats: {
        Explore: { success: 10, errors: 0 },
        Plan: { success: 5, errors: 0 },
        'claude-code-guide': { success: 4, errors: 0 },
        'code-reviewer': { success: 3, errors: 1 },
        'general-purpose': { success: 2, errors: 0 },
      },
      skill_stats: {
        commit: { success: 5, errors: 0 },
        'review-pr': { success: 3, errors: 0 },
        bugfix: { success: 2, errors: 1 },
        'add-session-card': { success: 1, errors: 0 },
      },
    },
    loading: false,
  },
};

export const SingleItem: Story = {
  args: {
    data: {
      agent_invocations: 1,
      skill_invocations: 0,
      agent_stats: {
        Explore: { success: 1, errors: 0 },
      },
      skill_stats: {},
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

export const Empty: Story = {
  args: {
    data: {
      agent_invocations: 0,
      skill_invocations: 0,
      agent_stats: {},
      skill_stats: {},
    },
    loading: false,
  },
};
