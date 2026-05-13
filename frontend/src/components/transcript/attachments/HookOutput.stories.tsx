import type { Meta, StoryObj } from '@storybook/react-vite';
import { HookSuccessOutput, HookBlockingError } from './HookOutput';

const meta: Meta<typeof HookSuccessOutput> = {
  title: 'Transcript/Attachments/HookOutput',
  component: HookSuccessOutput,
  parameters: { layout: 'padded' },
};
export default meta;

type SuccessStory = StoryObj<typeof HookSuccessOutput>;

export const Success: SuccessStory = {
  args: {
    attachment: {
      type: 'hook_success',
      hookName: 'SessionStart:startup',
      hookEvent: 'SessionStart',
      toolUseID: 'tool-1',
      command: '/home/user/.local/bin/example-hook session-start',
      stdout: '{"continue":true,"stopReason":"","suppressOutput":false}\n',
      stderr: '=== Starting Sync Daemon ===\n\nSession: session-1\nSync daemon started in background\n',
      exitCode: 0,
      durationMs: 31,
    },
  },
};

export const SuccessEmptyStreams: SuccessStory = {
  args: {
    attachment: {
      type: 'hook_success',
      hookName: 'AfterEdit:format',
      hookEvent: 'AfterEdit',
      toolUseID: 'tool-2',
      command: '/usr/bin/prettier --write file.ts',
      stdout: '',
      stderr: '',
      exitCode: 0,
      durationMs: 142,
    },
  },
};

type BlockingStory = StoryObj<typeof HookBlockingError>;

export const Blocking: BlockingStory = {
  render: (args) => <HookBlockingError {...args} />,
  args: {
    attachment: {
      type: 'hook_blocking_error',
      hookName: 'PreToolUse:Bash',
      hookEvent: 'PreToolUse',
      toolUseID: 'tool-3',
      blockingError: {
        blockingError:
          '✓ Linking this commit to your session. Add this trailer to the end of your commit message (after any other trailers like Co-Authored-By):\n\n    Example-Link: https://example.com/sessions/session-1\n\nIMPORTANT: Copy this line verbatim.',
        command: '/home/user/.local/bin/example-hook pre-tool-use',
      },
    },
  },
};
