import type { Meta, StoryObj } from '@storybook/react-vite';
import QueuedCommand from './QueuedCommand';

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
