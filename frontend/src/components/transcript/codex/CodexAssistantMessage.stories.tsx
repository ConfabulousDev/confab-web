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

// CF-388: assistant-side image rendering (`output_image` content block).
// Uses the same 80x80 checkerboard PNG as the user-message story for parity.
const CHECKERBOARD_PNG =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAFAAAABQCAYAAACOEfKtAAAA6ElEQVR4nO3XwQnDMAAEQdeT/iE9uKmkBcM+7EtGoO8ize+O1/v8XLlXz7/1jqc/8Ok9gAABTvcAAgQ43QMI8GbAX/nIXT2AsQcw9gDGHsDYAxh7AGMPYOwBjD2AsWfKxR5AgACnewABApzuAQQIcLpnysUewNgDGHsAYw9g7AGMPYCxBzD2AMYewNgz5WIPIECA0z2AAAFO9wACBDjdM+ViD2DsAYw9gLEHMPYAxh7A2AMYewBjD2DsmXKxBxAgwOkeQIAAp3sAAQKc7plysQcw9gDGHsDYAxh7AGMPYOwBjD2AsQcw9r7VCh/edp941wAAAABJRU5ErkJggg==';

export const WithImage: Story = {
  args: {
    item: item({
      text: 'Here is the rendered diagram you asked for:',
      images: [CHECKERBOARD_PNG],
    }),
  },
};

// ---------------------------------------------------------------------------
// CF-362 — cost-mode badges
// ---------------------------------------------------------------------------

export const CostModeFinal: Story = {
  args: {
    item: item({
      usage: { input_tokens: 12_345, output_tokens: 1_200 },
    }),
    isCostMode: true,
    messageCost: 0.0274,
  },
};

export const CostModeWithCache: Story = {
  args: {
    item: item({
      usage: {
        input_tokens: 80_000,
        cached_input_tokens: 50_000,
        output_tokens: 3_500,
      },
    }),
    isCostMode: true,
    messageCost: 0.0469,
  },
};

export const CostModeWithReasoning: Story = {
  args: {
    item: item({
      phase: 'commentary',
      text: '*Thinking through the configuration changes...*',
      model: 'o3',
      usage: {
        input_tokens: 5_000,
        output_tokens: 400,
        reasoning_output_tokens: 2_000,
      },
    }),
    isCostMode: true,
    messageCost: 0.029,
  },
};
