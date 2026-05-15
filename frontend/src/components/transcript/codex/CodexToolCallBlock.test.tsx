// Component tests for CodexToolCallBlock — locks the dispatch contract per
// tool name and the rendering pipeline upgrade from CF-358 (BashOutput for
// exec, CodeBlock with language="diff" for apply_patch, expanded CodeBlocks
// for generic tools, no `<details>` wrappers).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import CodexToolCallBlock from './CodexToolCallBlock';
import type { CodexToolCallItem } from '@/types/codexRenderItem';

function execCommandItem(overrides: Partial<CodexToolCallItem> = {}): CodexToolCallItem {
  return {
    kind: 'tool_call',
    lineId: '0',
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
    expect(screen.getByText(/pwd/)).toBeInTheDocument();
    expect(screen.getByText(/\/tmp\/proj/)).toBeInTheDocument();
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
    const { container } = render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          lineId: '0',
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
    // The leaf filename appears inside the file-list summary <ul>. The patch
    // body + raw output also mention it, so scope the assertion to the list.
    const fileList = container.querySelector('ul');
    expect(fileList).not.toBeNull();
    expect(fileList?.textContent).toContain('codex-support.md');
  });

  it('renders web_search_call with query chips', () => {
    render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          lineId: '0',
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
          lineId: '0',
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

  // ---------------------------------------------------------------------------
  // CF-357 — selection contract
  // ---------------------------------------------------------------------------

  it('applies the selected class when isSelected is true', () => {
    const { container } = render(<CodexToolCallBlock item={execCommandItem()} isSelected />);
    expect(container.firstChild).toHaveClass(/selected/);
  });

  it('does not apply the selected class by default', () => {
    const { container } = render(<CodexToolCallBlock item={execCommandItem()} />);
    expect(container.firstChild).not.toHaveClass(/selected/);
  });

  // ---------------------------------------------------------------------------
  // CF-358 — content rendering parity
  // ---------------------------------------------------------------------------

  it('strips ANSI escape codes from exec_command output', () => {
    const { container } = render(
      <CodexToolCallBlock
        item={execCommandItem({ rawOutput: '\x1b[31merror line\x1b[0m\nsecond line' })}
      />,
    );
    // No raw escape characters survive into the DOM text content.
    expect(container.textContent).not.toContain('\x1b[');
    expect(container.textContent).toContain('error line');
    expect(container.textContent).toContain('second line');
  });

  it('routes exec_command output through BashOutput (terminal styling)', () => {
    const { container } = render(<CodexToolCallBlock item={execCommandItem()} />);
    // BashOutput.module.css exports a `.bashOutput` class on the container.
    // CSS Modules produces a unique class name that includes the source name.
    const bash = container.querySelector('[class*="bashOutput"]');
    expect(bash).not.toBeNull();
  });

  it('applies error styling to BashOutput container on non-zero exit', () => {
    const { container } = render(
      <CodexToolCallBlock
        item={execCommandItem({
          execMetadata: { exitCode: 1, wallTimeMs: 200 },
          rawOutput: 'boom',
        })}
      />,
    );
    const bash = container.querySelector('[class*="bashOutput"]');
    expect(bash).not.toBeNull();
    // `.error` from BashOutput.module.css must be present on (or under) the
    // container when exitCode !== 0.
    expect(bash?.className).toMatch(/error/i);
  });

  it('renders apply_patch raw diff with a prism language-diff class (no expand toggle)', () => {
    const { container } = render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          lineId: '0',
          timestamp: '2026-05-13T01:00:00Z',
          toolName: 'apply_patch',
          callId: 'call_patch_diff',
          rawInput: '--- a/foo.ts\n+++ b/foo.ts\n@@\n-old\n+new\n',
          structuredOutput: {
            success: true,
            changes: { '/proj/foo.ts': { type: 'update' } },
          },
          status: 'completed',
        }}
      />,
    );
    const diffCode = container.querySelector('code[class*="language-diff"]');
    expect(diffCode).not.toBeNull();
    // The raw diff text shows without clicking any expansion button.
    expect(diffCode?.textContent).toContain('-old');
    expect(diffCode?.textContent).toContain('+new');
  });

  it('generic tool renders rawInput JSON via CodeBlock (language-json, expanded)', () => {
    const { container } = render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          lineId: '0',
          timestamp: '2026-05-13T01:00:00Z',
          toolName: 'future_tool',
          callId: 'call_future',
          rawInput: { k: 'v', n: 1 },
          rawOutput: 'plain text result',
          status: 'completed',
        }}
      />,
    );
    expect(container.querySelector('code[class*="language-json"]')).not.toBeNull();
    expect(container.querySelector('code[class*="language-plain"]')).not.toBeNull();
    // No <summary> means no collapsed <details> wrapper — content is expanded.
    expect(container.querySelector('summary')).toBeNull();
  });

  it('generic tool renders string rawInput that parses as JSON with language-json', () => {
    const { container } = render(
      <CodexToolCallBlock
        item={{
          kind: 'tool_call',
          lineId: '0',
          timestamp: '2026-05-13T01:00:00Z',
          toolName: 'future_tool_str',
          callId: 'call_future_str',
          rawInput: '{"k":"v"}',
          rawOutput: '',
          status: 'completed',
        }}
      />,
    );
    expect(container.querySelector('code[class*="language-json"]')).not.toBeNull();
  });

  // ---------------------------------------------------------------------------
  // CF-378 — empty exec_command output
  // ---------------------------------------------------------------------------

  it('renders empty exec_command output via NoOutputIndicator, not BashOutput', () => {
    const { container } = render(
      <CodexToolCallBlock
        item={execCommandItem({ rawOutput: '', status: 'completed' })}
      />,
    );
    expect(screen.getByText(/^no output$/i)).toBeInTheDocument();
    // Strictly "no output" — not the "pending — no output yet" label.
    expect(screen.queryByText(/pending/i)).toBeNull();
    expect(container.querySelector('[class*="bashOutput"]')).toBeNull();
    expect(screen.getByText(/pwd/)).toBeInTheDocument();
    expect(screen.getByText(/exit\s*0/i)).toBeInTheDocument();
  });

  it('renders pending exec_command with the "pending — no output yet" label', () => {
    render(
      <CodexToolCallBlock
        item={execCommandItem({
          status: 'pending',
          rawOutput: undefined,
          execMetadata: undefined,
        })}
      />,
    );
    expect(screen.getByText(/pending\s*—\s*no output yet/i)).toBeInTheDocument();
  });

  // ---------------------------------------------------------------------------
  // CF-360 — deep-link target + row-actions + copy-text composition
  // ---------------------------------------------------------------------------

  it('applies the deepLinkTarget class when isDeepLinkTarget is true', () => {
    const { container } = render(
      <CodexToolCallBlock item={execCommandItem()} isDeepLinkTarget />,
    );
    expect(container.firstChild).toHaveClass(/deepLinkTarget/);
  });

  it('renders a copy-link button when sessionId is provided', () => {
    render(<CodexToolCallBlock item={execCommandItem()} sessionId="abc" />);
    expect(screen.getByLabelText(/copy link/i)).toBeInTheDocument();
  });

  it('omits row-actions when sessionId is absent', () => {
    render(<CodexToolCallBlock item={execCommandItem()} />);
    expect(screen.queryByLabelText(/copy link/i)).toBeNull();
  });
});
