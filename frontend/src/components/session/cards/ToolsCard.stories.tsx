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
      <div style={{ width: '280px' }}>
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
      tool_breakdown: {
        Read: 20,
        Bash: 15,
        Edit: 8,
        Grep: 2,
      },
      error_count: 0,
    },
    loading: false,
  },
};

export const WithErrors: Story = {
  args: {
    data: {
      total_calls: 32,
      tool_breakdown: {
        Bash: 18,
        Read: 10,
        Write: 4,
      },
      error_count: 3,
    },
    loading: false,
  },
};

export const SingleTool: Story = {
  args: {
    data: {
      total_calls: 12,
      tool_breakdown: {
        Read: 12,
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
      tool_breakdown: {
        Bash: 45,
        Read: 35,
        Edit: 30,
        Grep: 20,
        Glob: 15,
        Write: 5,
      },
      error_count: 2,
    },
    loading: false,
  },
};

export const NoTools: Story = {
  args: {
    data: {
      total_calls: 0,
      tool_breakdown: {},
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
