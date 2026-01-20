import type { Meta, StoryObj } from '@storybook/react-vite';
import { TrendsToolsCard } from './TrendsToolsCard';

const meta: Meta<typeof TrendsToolsCard> = {
  title: 'Trends/Cards/TrendsToolsCard',
  component: TrendsToolsCard,
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
type Story = StoryObj<typeof TrendsToolsCard>;

export const Default: Story = {
  args: {
    data: {
      total_calls: 2500,
      total_errors: 50,
      tool_stats: {
        Read: { success: 800, errors: 5 },
        Write: { success: 400, errors: 10 },
        Edit: { success: 350, errors: 8 },
        Bash: { success: 600, errors: 30 },
        Grep: { success: 200, errors: 2 },
        Glob: { success: 150, errors: 0 },
      },
    },
  },
};

export const HighErrors: Story = {
  args: {
    data: {
      total_calls: 500,
      total_errors: 150,
      tool_stats: {
        Bash: { success: 100, errors: 80 },
        Write: { success: 150, errors: 50 },
        Edit: { success: 100, errors: 20 },
      },
    },
  },
};

export const MCPTools: Story = {
  args: {
    data: {
      total_calls: 200,
      total_errors: 5,
      tool_stats: {
        Read: { success: 50, errors: 0 },
        'mcp__linear-server__list_issues': { success: 30, errors: 2 },
        'mcp__linear-server__create_issue': { success: 20, errors: 1 },
        'mcp__github__create_pr': { success: 15, errors: 2 },
      },
    },
  },
};

export const ManyTools: Story = {
  args: {
    data: {
      total_calls: 5000,
      total_errors: 100,
      tool_stats: {
        Read: { success: 1200, errors: 10 },
        Write: { success: 800, errors: 20 },
        Edit: { success: 600, errors: 15 },
        Bash: { success: 500, errors: 40 },
        Grep: { success: 400, errors: 5 },
        Glob: { success: 350, errors: 0 },
        Task: { success: 300, errors: 5 },
        WebFetch: { success: 200, errors: 3 },
        WebSearch: { success: 150, errors: 2 },
        TodoWrite: { success: 100, errors: 0 },
        NotebookEdit: { success: 80, errors: 0 },
        AskUserQuestion: { success: 50, errors: 0 },
      },
    },
  },
};

export const NoErrors: Story = {
  args: {
    data: {
      total_calls: 100,
      total_errors: 0,
      tool_stats: {
        Read: { success: 50, errors: 0 },
        Write: { success: 30, errors: 0 },
        Edit: { success: 20, errors: 0 },
      },
    },
  },
};

export const NoCalls: Story = {
  args: {
    data: {
      total_calls: 0,
      total_errors: 0,
      tool_stats: {},
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};
