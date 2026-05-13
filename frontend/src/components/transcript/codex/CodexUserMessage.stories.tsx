import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexUserMessage from './CodexUserMessage';
import type { CodexUserItem } from '@/types/codexRenderItem';

const meta: Meta<typeof CodexUserMessage> = {
  title: 'Transcript/Codex/CodexUserMessage',
  component: CodexUserMessage,
};

export default meta;
type Story = StoryObj<typeof CodexUserMessage>;

function item(text: string): CodexUserItem {
  return {
    kind: 'user',
    timestamp: '2026-05-13T18:00:00Z',
    text,
  };
}

export const Short: Story = {
  args: { item: item('add the linear mcp to my codex config') },
};

export const Multiline: Story = {
  args: {
    item: item(
      'Here is a longer prompt that wraps over\nmultiple lines so we can verify how\nwhitespace is preserved in the chat row.',
    ),
  },
};
