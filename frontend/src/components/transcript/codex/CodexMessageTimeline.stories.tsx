import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexMessageTimeline from './CodexMessageTimeline';
import type { CodexRenderItem } from '@/types/codexRenderItem';

const meta: Meta<typeof CodexMessageTimeline> = {
  title: 'Transcript/Codex/CodexMessageTimeline',
  component: CodexMessageTimeline,
  parameters: { layout: 'fullscreen' },
};

export default meta;
type Story = StoryObj<typeof CodexMessageTimeline>;

const sample: CodexRenderItem[] = [
  {
    kind: 'user',
    timestamp: '2026-05-13T18:00:00Z',
    text: 'add the linear mcp to my codex config',
  },
  {
    kind: 'reasoning_hidden',
    timestamp: '2026-05-13T18:00:01Z',
  },
  {
    kind: 'assistant',
    timestamp: '2026-05-13T18:00:02Z',
    text: "I'll check how this repo manages MCP entries so I can add Linear in the same style.",
    phase: 'commentary',
    model: 'gpt-5',
  },
  {
    kind: 'tool_call',
    timestamp: '2026-05-13T18:00:03Z',
    toolName: 'exec_command',
    callId: 'c1',
    rawInput: { cmd: 'pwd', workdir: '/Users/dev/example-project' },
    rawOutput: '/Users/dev/example-project',
    status: 'completed',
    execMetadata: { exitCode: 0, wallTimeMs: 700 },
  },
  {
    kind: 'tool_call',
    timestamp: '2026-05-13T18:00:10Z',
    toolName: 'apply_patch',
    callId: 'c2',
    rawInput: '*** Begin Patch\n*** Add File: docs/codex-support.md\n+# Plan\n*** End Patch',
    rawOutput: '{"output":"Success."}',
    structuredOutput: {
      success: true,
      changes: {
        '/proj/docs/codex-support.md': { type: 'add', content: '# Plan' },
      },
    },
    status: 'completed',
  },
  {
    kind: 'assistant',
    timestamp: '2026-05-13T18:00:11Z',
    text: 'Added the Linear MCP entry to your Codex config.\n\nReload the session for the change to take effect.',
    phase: 'final',
    model: 'gpt-5',
  },
  {
    kind: 'turn_separator',
    timestamp: '2026-05-13T18:00:11Z',
    turnIndex: 1,
    durationMs: 11000,
    timeToFirstTokenMs: 1704,
  },
  {
    kind: 'user',
    timestamp: '2026-05-13T18:01:00Z',
    text: 'look at CF-342',
  },
  {
    kind: 'tool_call',
    timestamp: '2026-05-13T18:01:05Z',
    toolName: 'web_search_call',
    callId: 'c3',
    rawInput: {
      type: 'search',
      query: 'site:openai.com codex cli sessions jsonl format',
      queries: ['site:openai.com codex cli sessions jsonl format', 'openai codex cli rollout schema'],
    },
    status: 'completed',
  },
  {
    kind: 'assistant',
    timestamp: '2026-05-13T18:01:06Z',
    text: 'CF-342 is the umbrella ticket for incremental Codex support. The plan splits work across CLI, backend, and frontend.',
    phase: 'final',
    model: 'gpt-5',
  },
  {
    kind: 'turn_separator',
    timestamp: '2026-05-13T18:01:06Z',
    turnIndex: 2,
    durationMs: 6000,
    timeToFirstTokenMs: 900,
  },
  {
    kind: 'compacted',
    timestamp: '2026-05-13T18:02:00Z',
    replacementCount: 2,
  },
];

function Frame({ children }: { children: React.ReactNode }) {
  return <div style={{ height: '600px', width: '100%' }}>{children}</div>;
}

export const FullSession: Story = {
  render: () => (
    <Frame>
      <CodexMessageTimeline items={sample} />
    </Frame>
  ),
};

export const Empty: Story = {
  render: () => (
    <Frame>
      <CodexMessageTimeline items={[]} />
    </Frame>
  ),
};
