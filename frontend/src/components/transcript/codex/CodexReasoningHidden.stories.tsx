import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexReasoningHidden from './CodexReasoningHidden';

const meta: Meta<typeof CodexReasoningHidden> = {
  title: 'Transcript/Codex/CodexReasoningHidden',
  component: CodexReasoningHidden,
};

export default meta;
type Story = StoryObj<typeof CodexReasoningHidden>;

export const Default: Story = {
  args: {
    item: { kind: 'reasoning_hidden', timestamp: '2026-05-13T18:00:01Z' },
  },
};
