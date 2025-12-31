import type { Meta, StoryObj } from '@storybook/react-vite';
import { SkillsCard } from './SkillsCard';

const meta: Meta<typeof SkillsCard> = {
  title: 'Session/Cards/SkillsCard',
  component: SkillsCard,
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
type Story = StoryObj<typeof SkillsCard>;

export const Default: Story = {
  args: {
    data: {
      total_invocations: 6,
      skill_stats: {
        commit: { success: 3, errors: 0 },
        'codebase-maintenance': { success: 2, errors: 0 },
        bugfix: { success: 1, errors: 0 },
      },
    },
    loading: false,
  },
};

export const WithErrors: Story = {
  args: {
    data: {
      total_invocations: 10,
      skill_stats: {
        commit: { success: 5, errors: 1 },
        'review-pr': { success: 2, errors: 2 },
      },
    },
    loading: false,
  },
};

export const SingleSkill: Story = {
  args: {
    data: {
      total_invocations: 4,
      skill_stats: {
        commit: { success: 4, errors: 0 },
      },
    },
    loading: false,
  },
};

export const ManySkills: Story = {
  args: {
    data: {
      total_invocations: 20,
      skill_stats: {
        commit: { success: 8, errors: 0 },
        'codebase-maintenance': { success: 4, errors: 1 },
        bugfix: { success: 3, errors: 0 },
        'add-session-card': { success: 2, errors: 0 },
        'review-pr': { success: 1, errors: 1 },
      },
    },
    loading: false,
  },
};

export const AllErrors: Story = {
  args: {
    data: {
      total_invocations: 3,
      skill_stats: {
        commit: { success: 0, errors: 2 },
        bugfix: { success: 0, errors: 1 },
      },
    },
    loading: false,
  },
};

export const NoSkills: Story = {
  args: {
    data: {
      total_invocations: 0,
      skill_stats: {},
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'When no skills are used, the card is not rendered (returns null)',
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
