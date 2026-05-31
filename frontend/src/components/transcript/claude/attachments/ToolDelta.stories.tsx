import type { Meta, StoryObj } from '@storybook/react-vite';
import ToolDelta from './ToolDelta';

const meta: Meta<typeof ToolDelta> = {
  title: 'Transcript/Attachments/ToolDelta',
  component: ToolDelta,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof ToolDelta>;

export const DeferredTools: Story = {
  args: {
    attachment: {
      type: 'deferred_tools_delta',
      addedNames: [
        'AskUserQuestion', 'CronCreate', 'CronDelete', 'CronList',
        'EnterPlanMode', 'EnterWorktree', 'ExitPlanMode', 'ExitWorktree',
        'LSP', 'Monitor', 'NotebookEdit', 'PushNotification',
        'RemoteTrigger', 'TaskCreate', 'TaskGet', 'TaskList',
        'TaskOutput', 'TaskStop', 'TaskUpdate', 'WebFetch', 'WebSearch',
      ],
      removedNames: [],
      addedLines: [],
    },
  },
};

export const McpInstructions: Story = {
  args: {
    attachment: {
      type: 'mcp_instructions_delta',
      addedNames: ['example-mcp-server'],
      removedNames: [],
      addedBlocks: ['## example-mcp-server\nWhen passing string values, send the content directly without escape sequences.'],
    },
  },
};

export const WithRemovals: Story = {
  args: {
    attachment: {
      type: 'deferred_tools_delta',
      addedNames: ['WebSearch', 'WebFetch'],
      removedNames: ['LegacyTool'],
    },
  },
};
