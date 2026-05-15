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

// CF-378: empty stdout (e.g. `git status --short` on a clean tree) renders
// the "no output" indicator, not an empty BashOutput frame.
export const ExecEmptyOutput: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'exec_command',
      callId: 'call_exec_empty',
      rawInput: { cmd: 'git status --short', workdir: '/Users/dev/example-project' },
      rawOutput: '',
      status: 'completed',
      execMetadata: { exitCode: 0, wallTimeMs: 18 },
    } satisfies CodexToolCallItem,
  },
};

// Long output now scrolls inside BashOutput's frame (max-height 400px) rather
// than collapsing behind a `Show all` toggle — that affordance was dropped in
// CF-358 to mirror Claude's Bash-tool rendering.
export const ExecScrollableOutput: Story = {
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

// Exercises ANSI stripping inside BashOutput — colour escapes disappear and the
// terminal frame stays clean.
export const ExecWithAnsi: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'exec_command',
      callId: 'call_exec_ansi',
      rawInput: { cmd: 'npm test', workdir: '/tmp/proj' },
      rawOutput: [
        '\x1b[32m✓\x1b[0m passes one',
        '\x1b[32m✓\x1b[0m passes two',
        '\x1b[31m✗\x1b[0m fails three',
      ].join('\n'),
      status: 'failed',
      execMetadata: { exitCode: 1, wallTimeMs: 1200 },
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

// Multi-file unified diff so the Prism `diff` highlighting (additions in green,
// deletions in red) is exercised end-to-end.
export const ApplyPatchDiff: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'apply_patch',
      callId: 'call_patch_diff',
      rawInput: [
        '--- a/src/foo.ts',
        '+++ b/src/foo.ts',
        '@@ -1,4 +1,5 @@',
        ' export function foo(): string {',
        '-  return "hello";',
        '+  // CF-358: terser greeting',
        '+  return "hi";',
        ' }',
        '--- a/README.md',
        '+++ b/README.md',
        '@@ -3,3 +3,3 @@',
        '-old line',
        '+new line',
      ].join('\n'),
      rawOutput: '{"output":"Success."}',
      structuredOutput: {
        success: true,
        changes: {
          '/proj/src/foo.ts': { type: 'update' },
          '/proj/README.md': { type: 'update' },
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

// Generic tool with a JSON-shaped string rawInput — should render as
// syntax-highlighted JSON via the shared `tryParseAsJson` fallback.
export const GenericJsonInput: Story = {
  args: {
    item: {
      kind: 'tool_call',
      timestamp: '2026-05-13T18:00:00Z',
      toolName: 'future_tool_json',
      callId: 'call_future_json',
      rawInput: '{"target":"/tmp","options":{"recursive":true,"depth":3}}',
      rawOutput: 'scanned 42 files',
      status: 'completed',
    } satisfies CodexToolCallItem,
  },
};
