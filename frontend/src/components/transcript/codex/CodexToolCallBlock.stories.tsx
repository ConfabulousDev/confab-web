import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexToolCallBlock from './CodexToolCallBlock';
import type { CodexToolCallItem } from '@/types/codexRenderItem';

const meta: Meta<typeof CodexToolCallBlock> = {
  title: 'Transcript/Codex/CodexToolCallBlock',
  component: CodexToolCallBlock,
};

export default meta;
type Story = StoryObj<typeof CodexToolCallBlock>;

export const ExecSuccess: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'exec_command',
      callId: 'call_exec_ok',
      rawInput: { cmd: 'pwd', workdir: '/Users/dev/example-project' },
      rawOutput: '/Users/dev/example-project',
      status: 'completed',
      execMetadata: { exitCode: 0, wallTimeMs: 700 },
    } satisfies CodexToolCallItem,
  },
};

export const ExecFailure: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'exec_command',
      callId: 'call_exec_fail',
      rawInput: { cmd: 'cat nonexistent.txt', workdir: '/tmp' },
      rawOutput: 'cat: nonexistent.txt: No such file or directory',
      status: 'failed',
      execMetadata: { exitCode: 1, wallTimeMs: 200 },
    } satisfies CodexToolCallItem,
  },
};

export const ExecLongOutput: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'exec_command',
      callId: 'call_exec_long',
      rawInput: { cmd: 'ls -la', workdir: '/tmp' },
      rawOutput: Array.from({ length: 200 }, (_, i) => `line ${i + 1}`).join('\n'),
      status: 'completed',
      execMetadata: { exitCode: 0, wallTimeMs: 50 },
    } satisfies CodexToolCallItem,
  },
};

export const ApplyPatch: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'apply_patch',
      callId: 'call_patch',
      rawInput: '*** Begin Patch\n*** Add File: docs/codex-support.md\n+# Plan\n*** End Patch',
      rawOutput: '{"output":"Success."}',
      structuredOutput: {
        success: true,
        changes: {
          '/proj/docs/codex-support.md': { type: 'add', content: '# Plan' },
          '/proj/README.md': { type: 'update', content: 'updated section' },
        },
      },
      status: 'completed',
    } satisfies CodexToolCallItem,
  },
};

export const WebSearch: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'web_search_call',
      callId: 'call_search',
      rawInput: {
        type: 'search',
        query: 'codex cli rollout',
        queries: ['codex cli rollout', 'openai codex jsonl format'],
      },
      status: 'completed',
    } satisfies CodexToolCallItem,
  },
};

export const Pending: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'exec_command',
      callId: 'call_pending',
      rawInput: { cmd: 'sleep 5', workdir: '/tmp' },
      status: 'pending',
    } satisfies CodexToolCallItem,
  },
};

export const UnknownTool: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'future_tool',
      callId: 'call_future',
      rawInput: { some: 'shape', nested: { a: 1 } },
      rawOutput: 'some output',
      status: 'completed',
    } satisfies CodexToolCallItem,
  },
};
