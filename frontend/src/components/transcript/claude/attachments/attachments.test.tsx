import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { HookSuccessOutput, HookBlockingError } from './HookOutput';
import EditedFileSnippet from './EditedFileSnippet';
import QueuedCommand from './QueuedCommand';
import ToolDelta from './ToolDelta';
import AwaySummary from './AwaySummary';
import type { SystemMessage } from '@/types';

describe('HookSuccessOutput', () => {
  it('renders stdout and stderr when non-empty', () => {
    render(
      <HookSuccessOutput
        attachment={{
          type: 'hook_success',
          hookName: 'SessionStart:startup',
          hookEvent: 'SessionStart',
          stdout: 'STDOUT_BODY',
          stderr: 'STDERR_BODY',
          exitCode: 0,
          durationMs: 31,
        }}
      />
    );
    expect(screen.getByText('STDOUT_BODY')).toBeInTheDocument();
    expect(screen.getByText('STDERR_BODY')).toBeInTheDocument();
    expect(screen.getByText('exit 0')).toBeInTheDocument();
    expect(screen.getByText('31ms')).toBeInTheDocument();
  });

  it('omits empty stdout/stderr', () => {
    render(
      <HookSuccessOutput
        attachment={{
          type: 'hook_success',
          hookName: 'h',
          stdout: '',
          stderr: '   \n',
        }}
      />
    );
    expect(screen.queryByText('stdout')).not.toBeInTheDocument();
    expect(screen.queryByText('stderr')).not.toBeInTheDocument();
  });
});

describe('HookBlockingError', () => {
  it('renders blockingError text in a labeled panel', () => {
    render(
      <HookBlockingError
        attachment={{
          type: 'hook_blocking_error',
          hookName: 'PreToolUse:Bash',
          hookEvent: 'PreToolUse',
          blockingError: {
            blockingError: 'Add the Confab-Link trailer to your commit message.',
            command: '/usr/local/bin/confab hook pre-tool-use',
          },
        }}
      />
    );
    expect(screen.getByText(/Confab-Link trailer/)).toBeInTheDocument();
    expect(screen.getByText(/Blocking error sent to model/i)).toBeInTheDocument();
    expect(screen.getByText('hook blocked')).toBeInTheDocument();
  });
});

describe('EditedFileSnippet', () => {
  it('strips cat -n prefixes and renders filename + snippet', () => {
    render(
      <EditedFileSnippet
        attachment={{
          type: 'edited_text_file',
          filename: '/home/user/notes.md',
          snippet: '     1\t# Notes\n     2\t\n     3\tHello there.',
        }}
      />
    );
    expect(screen.getByText('/home/user/notes.md')).toBeInTheDocument();
    // The line-number prefixes should be stripped before being passed to CodeBlock.
    expect(screen.queryByText(/^\s*1\t/)).not.toBeInTheDocument();
  });

  it('infers language from filename extension (.ts → typescript)', () => {
    const { container } = render(
      <EditedFileSnippet
        attachment={{
          type: 'edited_text_file',
          filename: 'file.ts',
          snippet: '     1\tconst x = 1;',
        }}
      />
    );
    expect(container.querySelector('.language-typescript')).toBeInTheDocument();
  });

  it('falls back to plain language for unknown extension', () => {
    const { container } = render(
      <EditedFileSnippet
        attachment={{
          type: 'edited_text_file',
          filename: 'something.unknownext',
          snippet: 'hello',
        }}
      />
    );
    expect(container.querySelector('.language-plain')).toBeInTheDocument();
  });
});

describe('QueuedCommand', () => {
  it('renders free-text prompt via markdown', () => {
    const { container } = render(
      <QueuedCommand
        attachment={{
          type: 'queued_command',
          prompt: '**check the build**',
          commandMode: 'prompt',
        }}
      />
    );
    // Markdown should produce <strong>
    expect(container.querySelector('strong')).toBeInTheDocument();
  });

  it('renders task-notification prompt verbatim in a <pre>', () => {
    const xml = '<task-notification><task-id>abc</task-id><status>completed</status></task-notification>';
    const { container } = render(
      <QueuedCommand
        attachment={{
          type: 'queued_command',
          prompt: xml,
          commandMode: 'task-notification',
        }}
      />
    );
    const pre = container.querySelector('pre');
    expect(pre).toBeInTheDocument();
    expect(pre?.textContent).toBe(xml);
  });
});

describe('ToolDelta', () => {
  it('renders deferred-tools header and chip lists for added + removed', () => {
    render(
      <ToolDelta
        attachment={{
          type: 'deferred_tools_delta',
          addedNames: ['WebFetch', 'WebSearch'],
          removedNames: ['Bash'],
        }}
      />
    );
    expect(screen.getByText('deferred tools')).toBeInTheDocument();
    expect(screen.getByText('WebFetch')).toBeInTheDocument();
    expect(screen.getByText('WebSearch')).toBeInTheDocument();
    expect(screen.getByText('Bash')).toBeInTheDocument();
  });

  it('renders mcp-instructions header for mcp_instructions_delta', () => {
    render(
      <ToolDelta
        attachment={{
          type: 'mcp_instructions_delta',
          addedNames: ['linear-server'],
          removedNames: [],
        }}
      />
    );
    expect(screen.getByText('mcp instructions')).toBeInTheDocument();
    expect(screen.getByText('linear-server')).toBeInTheDocument();
  });
});

describe('AwaySummary', () => {
  function makeSystemMessage(content: string | undefined): SystemMessage {
    return {
      type: 'system',
      uuid: 'sys-1',
      timestamp: '2026-04-20T22:35:57.594Z',
      parentUuid: null,
      isSidechain: false,
      userType: 'external',
      cwd: '/home/user',
      sessionId: 'session-1',
      version: '2.1.116',
      subtype: 'away_summary',
      content,
    };
  }

  it('renders markdown content', () => {
    const { container } = render(<AwaySummary message={makeSystemMessage('You stepped **away**.')} />);
    expect(container.querySelector('strong')).toBeInTheDocument();
  });

  it('renders nothing for empty content', () => {
    const { container } = render(<AwaySummary message={makeSystemMessage('')} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing for whitespace-only content', () => {
    const { container } = render(<AwaySummary message={makeSystemMessage('   \n  ')} />);
    expect(container.firstChild).toBeNull();
  });
});
