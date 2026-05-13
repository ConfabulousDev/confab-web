// Component tests for CodexToolCallBlock — locks the dispatch contract per
// tool name. Each branch is a separate spec-test row.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import CodexToolCallBlock from './CodexToolCallBlock';
import type { CodexToolCallItem } from '@/types/codexRenderItem';

function execCommandItem(overrides: Partial<CodexToolCallItem> = {}): CodexToolCallItem {
  return {
    kind: 'tool_call',
    timestamp: '2026-05-13T01:00:00Z',
    toolName: 'exec_command',
    callId: 'call_test_001',
    rawInput: { cmd: 'pwd', workdir: '/tmp/proj' },
    rawOutput: '/tmp/proj',
    status: 'completed',
    execMetadata: { exitCode: 0, wallTimeMs: 700 },
    ...overrides,
  };
}

describe('CodexToolCallBlock', () => {
  it('renders exec_command with command, output, and exit-code badge', () => {
    render(<CodexToolCallBlock item={execCommandItem()} />);
    // The command itself is visible.
    expect(screen.getByText(/pwd/)).toBeInTheDocument();
    // Output text is rendered.
    expect(screen.getByText(/\/tmp\/proj/)).toBeInTheDocument();
    // Exit-code badge ("exit 0") shows up somewhere in the rendered DOM.
    expect(screen.getByText(/exit\s*0/i)).toBeInTheDocument();
  });

  it('renders exec_command failure with non-zero exit code visible', () => {
    render(
      <CodexToolCallBlock
        item={execCommandItem({
          execMetadata: { exitCode: 1, wallTimeMs: 200 },
          rawOutput: 'error: something broke',
        })}
      />,
    );
    expect(screen.getByText(/exit\s*1/i)).toBeInTheDocument();
  });

  it('renders apply_patch with file-list summary', () => {
    render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          timestamp: '2026-05-13T01:00:00Z',
          toolName: 'apply_patch',
          callId: 'call_patch_001',
          rawInput: '*** Begin Patch\n*** Add File: docs/codex-support.md\n+content\n*** End Patch',
          rawOutput: '{"output":"Success. Updated the following files:\\nA docs/codex-support.md\\n"}',
          structuredOutput: {
            success: true,
            changes: {
              '/proj/docs/codex-support.md': { type: 'add', content: '# Plan' },
            },
          },
          status: 'completed',
        }}
      />,
    );
    // The file path or its leaf shows up in the rendered summary.
    expect(screen.getByText(/codex-support\.md/)).toBeInTheDocument();
  });

  it('renders web_search_call with query chips', () => {
    render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          timestamp: '2026-05-13T01:00:00Z',
          toolName: 'web_search_call',
          callId: 'call_search_001',
          rawInput: {
            type: 'search',
            query: 'codex cli rollout',
            queries: ['codex cli rollout', 'openai codex jsonl'],
          },
          status: 'completed',
        }}
      />,
    );
    expect(screen.getByText(/codex cli rollout/)).toBeInTheDocument();
    expect(screen.getByText(/openai codex jsonl/)).toBeInTheDocument();
  });

  it('renders an unknown tool name with generic "Tool: <name>" label', () => {
    render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          timestamp: '2026-05-13T01:00:00Z',
          toolName: 'future_tool_name',
          callId: 'call_unknown_001',
          rawInput: { some: 'shape' },
          rawOutput: 'some output',
          status: 'completed',
        }}
      />,
    );
    expect(screen.getByText(/future_tool_name/)).toBeInTheDocument();
  });

  it('renders a pending tool call with no-output indicator', () => {
    render(
      <CodexToolCallBlock
        item={execCommandItem({
          status: 'pending',
          rawOutput: undefined,
          execMetadata: undefined,
        })}
      />,
    );
    expect(screen.getByText(/pending|no output/i)).toBeInTheDocument();
  });
});
