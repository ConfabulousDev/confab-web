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
    lineId: '0',
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

// Exercises the full markdown rendering pipeline (CF-358): headings, lists,
// inline code, fenced code with Prism syntax highlighting.
// CF-360: deep-link landing variant exercises the .deepLinkTarget pulse.
export const WithDeepLinkTarget: Story = {
  args: {
    item: item({ text: 'this final answer is the deep-link target' }),
    sessionId: 'story-session',
    isDeepLinkTarget: true,
  },
};

export const MarkdownHeavy: Story = {
  args: {
    item: item({
      text: [
        '## Adding Linear MCP',
        '',
        'I’ll make the following changes:',
        '',
        '- Update `~/.codex/config.toml`',
        '- Add an `mcp_servers.linear` block',
        '- Restart codex',
        '',
        'Here is the new section:',
        '',
        '```toml',
        '[mcp_servers.linear]',
        'command = "npx"',
        'args = ["-y", "@linear/mcp"]',
        '```',
        '',
        'Then verify with `codex mcp list`.',
      ].join('\n'),
    }),
  },
};
