import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexAssistantMessage from './CodexAssistantMessage';
import type { CodexAssistantItem } from '@/types/codexRenderItem';

const meta: Meta<typeof CodexAssistantMessage> = {
  title: 'Transcript/Codex/CodexAssistantMessage',
  component: CodexAssistantMessage,
};

export default meta;
type Story = StoryObj<typeof CodexAssistantMessage>;

function item(overrides: Partial<CodexAssistantItem> = {}): CodexAssistantItem {
  return {
    kind: 'assistant',
    timestamp: '2026-05-13T18:00:00Z',
    text: "I'll check how this repo manages MCP entries so I can add Linear in the same style.",
    phase: 'final',
    model: 'gpt-5',
    ...overrides,
  };
}

export const Final: Story = {
  args: { item: item({ phase: 'final' }) },
};

export const Commentary: Story = {
  args: { item: item({ phase: 'commentary' }) },
};
