import type { Meta, StoryObj } from '@storybook/react-vite';
import QueuedCommand from './QueuedCommand';
import { buildClaudeAgentIndex } from '../claudeAgentIndex';
import { ClaudeThreadContext } from '../claudeThreadContext';
import { agentToolUse, asyncAgentResult, taskNotificationXml } from '@/test-fixtures/claudeSubagent';

const meta: Meta<typeof QueuedCommand> = {
  title: 'Transcript/Attachments/QueuedCommand',
  component: QueuedCommand,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof QueuedCommand>;

export const FreeTextPrompt: Story = {
  args: {
    attachment: {
      type: 'queued_command',
      prompt: 'After the build finishes, **check the lint results** and report back.',
      commandMode: 'prompt',
    },
  },
};

export const TaskNotification: Story = {
  args: {
    attachment: {
      type: 'queued_command',
      prompt:
        '<task-notification>\n' +
        '<task-id>example-task-1</task-id>\n' +
        '<tool-use-id>tool-1</tool-use-id>\n' +
        '<output-file>/tmp/example/tasks/example-task-1.output</output-file>\n' +
        '<status>completed</status>\n' +
        '<summary>Background command "Build" completed (exit code 0)</summary>\n' +
        '</task-notification>',
      commandMode: 'task-notification',
    },
  },
};

// et0r D5: a task-notification for a subagent the timeline has indexed renders
// the "Subagent finished" card (raw XML collapsed underneath).
const subagentIndex = buildClaudeAgentIndex([
  agentToolUse({ uuid: 'story-u1', toolUseId: 'toolu_story', description: 'Simplify the Go changes' }),
  asyncAgentResult({ uuid: 'story-r1', toolUseId: 'toolu_story', agentId: 'example-agent-1', description: 'Simplify the Go changes' }),
]);

export const SubagentFinishedCard: Story = {
  decorators: [
    (Story) => (
      <ClaudeThreadContext.Provider
        value={{ agentIndex: subagentIndex, onOpenThread: () => {}, activeThreadId: null }}
      >
        <Story />
      </ClaudeThreadContext.Provider>
    ),
  ],
  args: {
    attachment: {
      type: 'queued_command',
      prompt: taskNotificationXml({
        taskId: 'example-agent-1',
        toolUseId: 'toolu_story',
        summary: 'Agent "Simplify the Go changes" finished',
        result: 'Changed one comment; gofmt and go vet are clean.',
      }),
      commandMode: 'task-notification',
    },
  },
};
