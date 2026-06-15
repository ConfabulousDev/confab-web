import type { Meta, StoryObj } from '@storybook/react';
import BashOutput from './BashOutput';

const meta: Meta<typeof BashOutput> = {
  title: 'Transcript/BashOutput',
  component: BashOutput,
  parameters: {
    layout: 'padded',
    backgrounds: {
      default: 'app',
      values: [
        { name: 'app', value: 'var(--color-bg)' },
        { name: 'card', value: 'var(--color-bg-primary)' },
      ],
    },
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '800px', padding: '16px', background: 'var(--color-bg-primary)', borderRadius: '8px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof BashOutput>;

export const Plain: Story = {
  args: {
    command: 'ls -la',
    output: 'total 8\ndrwxr-xr-x  4 user  staff  128 Jun 15 10:00 .\n-rw-r--r--  1 user  staff   42 Jun 15 10:00 README.md',
  },
};

export const NonZeroExit: Story = {
  args: {
    command: './build.sh',
    output: 'error: build failed',
    exitCode: 1,
  },
};

export const ReturnCodeInterpretation: Story = {
  args: {
    command: 'grep -r needle .',
    output: '',
    exitCode: 1,
    returnCodeInterpretation: 'No matches found',
  },
};

export const Interrupted: Story = {
  args: {
    command: 'sleep 60',
    output: 'partial output before interrupt...',
    interrupted: true,
  },
};

export const PersistedOutput: Story = {
  args: {
    command: 'cat huge.log',
    output: '...first lines of a very large output that was truncated inline...',
    persistedOutputPath: '/Users/me/.claude/projects/p/27483ee0/tool-results/bw35rd3mb.txt',
    persistedOutputSize: 44276,
  },
};

export const NoOutputExpected: Story = {
  args: {
    command: 'npm run dev &',
    output: '',
    noOutputExpected: true,
  },
};

export const ImageOutput: Story = {
  args: {
    command: 'screencapture -x out.png && cat out.png',
    output: '<binary image data omitted>',
    isImage: true,
  },
};
