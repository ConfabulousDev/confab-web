import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsAgentsAndSkillsCard } from './TrendsAgentsAndSkillsCard';

const meta: Meta<typeof TrendsAgentsAndSkillsCard> = {
  title: 'Trends/Cards/TrendsAgentsAndSkillsCard',
  component: TrendsAgentsAndSkillsCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '400px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsAgentsAndSkillsCard>;

export const Default: Story = {
  args: {
    data: {
      total_agent_invocations: 45,
      total_skill_invocations: 20,
      agent_stats: {
        Explore: { success: 20, errors: 1 },
        Plan: { success: 12, errors: 0 },
        Bash: { success: 8, errors: 2 },
        'general-purpose': { success: 2, errors: 0 },
      },
      skill_stats: {
        commit: { success: 10, errors: 1 },
        'review-pr': { success: 5, errors: 0 },
        'frontend-design': { success: 4, errors: 0 },
      },
    },
  },
};

export const AgentsOnly: Story = {
  args: {
    data: {
      total_agent_invocations: 30,
      total_skill_invocations: 0,
      agent_stats: {
        Explore: { success: 15, errors: 2 },
        Plan: { success: 10, errors: 0 },
        Bash: { success: 3, errors: 0 },
      },
      skill_stats: {},
    },
  },
};

export const SkillsOnly: Story = {
  args: {
    data: {
      total_agent_invocations: 0,
      total_skill_invocations: 15,
      agent_stats: {},
      skill_stats: {
        commit: { success: 8, errors: 0 },
        'review-pr': { success: 5, errors: 1 },
        bugfix: { success: 1, errors: 0 },
      },
    },
  },
};

export const NoData: Story = {
  args: {
    data: null,
  },
};

export const ManyItems: Story = {
  args: {
    data: {
      total_agent_invocations: 120,
      total_skill_invocations: 80,
      agent_stats: {
        Explore: { success: 30, errors: 2 },
        Plan: { success: 25, errors: 1 },
        Bash: { success: 20, errors: 3 },
        'general-purpose': { success: 15, errors: 0 },
        'statusline-setup': { success: 8, errors: 0 },
        'claude-code-guide': { success: 6, errors: 2 },
        'test-runner': { success: 5, errors: 1 },
        'build-validator': { success: 3, errors: 0 },
      },
      skill_stats: {
        commit: { success: 25, errors: 1 },
        'review-pr': { success: 15, errors: 2 },
        'frontend-design': { success: 12, errors: 0 },
        bugfix: { success: 10, errors: 1 },
        'add-session-card': { success: 8, errors: 0 },
        'backend-maintenance': { success: 5, errors: 1 },
        'frontend-maintenance': { success: 3, errors: 0 },
      },
    },
  },
};
