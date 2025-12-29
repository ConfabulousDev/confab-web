import type { Meta, StoryObj } from '@storybook/react-vite';
import { ToolsCard } from './ToolsCard';

const meta: Meta<typeof ToolsCard> = {
  title: 'Session/Cards/ToolsCard',
  component: ToolsCard,
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
type Story = StoryObj<typeof ToolsCard>;

export const Default: Story = {
  args: {
    data: {
      total_calls: 45,
      tool_stats: {
        Read: { success: 20, errors: 0 },
        Bash: { success: 13, errors: 2 },
        Edit: { success: 8, errors: 0 },
        Grep: { success: 2, errors: 0 },
      },
      error_count: 2,
    },
    loading: false,
  },
};

export const WithErrors: Story = {
  args: {
    data: {
      total_calls: 32,
      tool_stats: {
        Bash: { success: 14, errors: 4 },
        Read: { success: 10, errors: 0 },
        Write: { success: 2, errors: 2 },
      },
      error_count: 6,
    },
    loading: false,
  },
};

export const SingleTool: Story = {
  args: {
    data: {
      total_calls: 12,
      tool_stats: {
        Read: { success: 12, errors: 0 },
      },
      error_count: 0,
    },
    loading: false,
  },
};

export const ManyTools: Story = {
  args: {
    data: {
      total_calls: 150,
      tool_stats: {
        Bash: { success: 43, errors: 2 },
        Read: { success: 35, errors: 0 },
        Edit: { success: 28, errors: 2 },
        Grep: { success: 20, errors: 0 },
        Glob: { success: 15, errors: 0 },
        Write: { success: 3, errors: 2 },
      },
      error_count: 6,
    },
    loading: false,
  },
};

export const LongToolNames: Story = {
  args: {
    data: {
      total_calls: 491,
      tool_stats: {
        Read: { success: 120, errors: 5 },
        Bash: { success: 108, errors: 7 },
        Edit: { success: 85, errors: 0 },
        Write: { success: 42, errors: 0 },
        Grep: { success: 35, errors: 0 },
        TodoWrite: { success: 28, errors: 2 },
        Glob: { success: 25, errors: 0 },
        WebFetch: { success: 12, errors: 1 },
        WebSearch: { success: 8, errors: 0 },
        TaskOutput: { success: 5, errors: 1 },
        Task: { success: 4, errors: 0 },
      },
      error_count: 16,
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Tests dynamic YAxis width with long tool names like TodoWrite, WebSearch, TaskOutput',
      },
    },
  },
};

export const AllErrors: Story = {
  args: {
    data: {
      total_calls: 5,
      tool_stats: {
        Bash: { success: 0, errors: 3 },
        Write: { success: 0, errors: 2 },
      },
      error_count: 5,
    },
    loading: false,
  },
};

export const NoTools: Story = {
  args: {
    data: {
      total_calls: 0,
      tool_stats: {},
      error_count: 0,
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'When no tools are used, the card is not rendered (returns null)',
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

export const WithMCPTools: Story = {
  args: {
    data: {
      total_calls: 89,
      tool_stats: {
        Bash: { success: 25, errors: 1 },
        Read: { success: 20, errors: 0 },
        Edit: { success: 15, errors: 0 },
        'mcp__linear-server__create_issue': { success: 8, errors: 0 },
        'mcp__linear-server__list_teams': { success: 6, errors: 0 },
        'mcp__linear-server__get_issue': { success: 5, errors: 1 },
        'mcp__github__create_pull_request': { success: 4, errors: 0 },
        Glob: { success: 3, errors: 0 },
        Task: { success: 2, errors: 0 },
      },
      error_count: 2,
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story:
          'MCP tool names are shortened to just the action (e.g., "create_issue"). Full name shown in tooltip on hover.',
      },
    },
  },
};
