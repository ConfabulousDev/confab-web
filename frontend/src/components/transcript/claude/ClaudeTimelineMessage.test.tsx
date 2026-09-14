import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserMessage, AssistantMessage, TranscriptLine, UnknownMessage } from '@/types';
import ClaudeTimelineMessage from './ClaudeTimelineMessage';
import { buildClaudeAgentIndex } from './claudeAgentIndex';
import { ClaudeThreadContext } from './claudeThreadContext';
import {
  agentToolUse,
  asyncAgentResult,
  taskNotificationMessage,
  subagentAssistantText,
} from '@/test-fixtures/claudeSubagent';

// Mock the useCopyToClipboard hook to capture copied text; keep the rest of
// the barrel real (btxt: unknown-message rows render ReportUnknownButton,
// which needs the real useAppConfig/AppConfigContext default).
const copiedTexts: string[] = [];
vi.mock('@/hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/hooks')>();
  return {
    ...actual,
    useCopyToClipboard: () => ({
      copy: (text: string) => {
        copiedTexts.push(text);
        return Promise.resolve();
      },
      copied: copiedTexts.length > 0,
      message: null,
    }),
  };
});

function createUserMessage(overrides: Partial<UserMessage> = {}): UserMessage {
  return {
    type: 'user',
    uuid: 'user-uuid-1',
    timestamp: '2025-01-15T10:00:00Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'session-1',
    version: '1.0.0',
    message: {
      role: 'user',
      content: 'Hello world',
    },
    ...overrides,
  };
}

function createAssistantMessage(overrides: Partial<AssistantMessage> = {}): AssistantMessage {
  return {
    type: 'assistant',
    uuid: 'assistant-uuid-1',
    timestamp: '2025-01-15T10:00:05Z',
    parentUuid: 'user-uuid-1',
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'session-1',
    version: '1.0.0',
    requestId: 'req-1',
    message: {
      model: 'claude-sonnet-4-20250514',
      id: 'msg-1',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'Hello! How can I help?' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 100,
        output_tokens: 50,
      },
    },
    ...overrides,
  };
}

function createUnknownMessage(overrides: Partial<UnknownMessage> = {}): UnknownMessage {
  return {
    type: 'agent-handoff',
    fromAgent: 'agent-1',
    toAgent: 'agent-2',
    timestamp: '2025-01-15T10:00:00Z',
    ...overrides,
  };
}

function createFileHistorySnapshot(): TranscriptLine {
  return {
    type: 'file-history-snapshot',
    messageId: 'snap-1',
    isSnapshotUpdate: false,
    snapshot: {
      messageId: 'snap-1',
      timestamp: '2025-01-15T10:00:00Z',
      trackedFileBackups: {},
    },
  };
}

const emptyToolNameMap = new Map<string, string>();

describe('ClaudeTimelineMessage', () => {
  beforeEach(() => {
    copiedTexts.length = 0;
  });

  describe('copy-link button', () => {
    it('renders link button for messages with uuid and sessionId', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          sessionId="test-session-id"
        />
      );

      expect(screen.getByLabelText('Copy link to message')).toBeInTheDocument();
    });

    it('does not render link button when sessionId is not provided', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
        />
      );

      expect(screen.queryByLabelText('Copy link to message')).not.toBeInTheDocument();
    });

    it('does not render link button for messages without uuid', () => {
      const message = createFileHistorySnapshot();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          sessionId="test-session-id"
        />
      );

      expect(screen.queryByLabelText('Copy link to message')).not.toBeInTheDocument();
    });

    it('copies clean deep-link URL to clipboard on click', async () => {
      const user = userEvent.setup();
      const message = createUserMessage({ uuid: 'my-msg-uuid' });

      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          sessionId="test-session-id"
        />
      );

      const linkBtn = screen.getByLabelText('Copy link to message');
      await user.click(linkBtn);

      expect(copiedTexts).toHaveLength(1);
      expect(copiedTexts[0]).toContain('/sessions/test-session-id?tab=transcript&msg=my-msg-uuid');
    });

    it('renders link button for assistant messages', () => {
      const message = createAssistantMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          sessionId="test-session-id"
        />
      );

      expect(screen.getByLabelText('Copy link to message')).toBeInTheDocument();
    });

    it('URL does not carry extra params (no PII leak)', async () => {
      const user = userEvent.setup();
      const message = createUserMessage({ uuid: 'test-uuid' });

      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          sessionId="session-abc"
        />
      );

      const linkBtn = screen.getByLabelText('Copy link to message');
      await user.click(linkBtn);

      const url = copiedTexts[0];
      // Should only have tab and msg params — no email, no other params
      expect(url).toMatch(/\?tab=transcript&msg=test-uuid$/);
    });
  });

  describe('deep-link highlight', () => {
    it('applies deepLinkTarget class when isDeepLinkTarget is true', () => {
      const message = createUserMessage();
      const { container } = render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isDeepLinkTarget={true}
        />
      );

      const messageEl = container.firstChild;
      expect(messageEl).toHaveClass(/deepLinkTarget/);
    });

    it('does not apply deepLinkTarget class when isDeepLinkTarget is false', () => {
      const message = createUserMessage();
      const { container } = render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isDeepLinkTarget={false}
        />
      );

      const messageEl = container.firstChild;
      expect(messageEl).not.toHaveClass(/deepLinkTarget/);
    });

    it('applies both selected and deepLinkTarget classes simultaneously', () => {
      const message = createUserMessage();
      const { container } = render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isSelected={true}
          isDeepLinkTarget={true}
        />
      );

      const messageEl = container.firstChild;
      expect(messageEl).toHaveClass(/selected/);
      expect(messageEl).toHaveClass(/deepLinkTarget/);
    });
  });

  describe('cost mode cache tokens', () => {
    it('shows cache write and hit when both are non-zero', () => {
      const message = createAssistantMessage({
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: {
            input_tokens: 15000,
            output_tokens: 2500,
            cache_creation_input_tokens: 5000,
            cache_read_input_tokens: 80000,
          },
        },
      });
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.42}
        />
      );

      expect(screen.getByText(/5.0k write/)).toBeInTheDocument();
      expect(screen.getByText(/80.0k hit/)).toBeInTheDocument();
    });

    it('shows only cache write when cache read is zero', () => {
      const message = createAssistantMessage({
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: {
            input_tokens: 15000,
            output_tokens: 2500,
            cache_creation_input_tokens: 5000,
            cache_read_input_tokens: 0,
          },
        },
      });
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.20}
        />
      );

      expect(screen.getByText(/5.0k write/)).toBeInTheDocument();
      expect(screen.queryByText(/hit/)).not.toBeInTheDocument();
    });

    it('shows only cache hit when cache write is zero', () => {
      const message = createAssistantMessage({
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: {
            input_tokens: 12000,
            output_tokens: 800,
            cache_creation_input_tokens: 0,
            cache_read_input_tokens: 45000,
          },
        },
      });
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.08}
        />
      );

      expect(screen.queryByText(/write/)).not.toBeInTheDocument();
      expect(screen.getByText(/45.0k hit/)).toBeInTheDocument();
    });

    it('shows no cache section when both are zero', () => {
      const message = createAssistantMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.01}
        />
      );

      expect(screen.queryByText(/write/)).not.toBeInTheDocument();
      expect(screen.queryByText(/hit/)).not.toBeInTheDocument();
    });
  });

  describe('copy message button (existing)', () => {
    it('still renders copy message button alongside link button', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          sessionId="test-session-id"
        />
      );

      expect(screen.getByLabelText('Copy message')).toBeInTheDocument();
      expect(screen.getByLabelText('Copy link to message')).toBeInTheDocument();
    });
  });

  describe('skip navigation buttons', () => {
    it('renders both skip buttons when both callbacks are provided', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          roleLabel="User"
          onSkipToNext={() => {}}
          onSkipToPrevious={() => {}}
        />
      );

      expect(screen.getByLabelText('Previous User message')).toBeInTheDocument();
      expect(screen.getByLabelText('Next User message')).toBeInTheDocument();
    });

    it('renders only next button when onSkipToPrevious is absent', () => {
      const message = createAssistantMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          roleLabel="Assistant"
          onSkipToNext={() => {}}
        />
      );

      expect(screen.queryByLabelText('Previous Assistant message')).not.toBeInTheDocument();
      expect(screen.getByLabelText('Next Assistant message')).toBeInTheDocument();
    });

    it('renders only previous button when onSkipToNext is absent', () => {
      const message = createAssistantMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          roleLabel="Assistant"
          onSkipToPrevious={() => {}}
        />
      );

      expect(screen.getByLabelText('Previous Assistant message')).toBeInTheDocument();
      expect(screen.queryByLabelText('Next Assistant message')).not.toBeInTheDocument();
    });

    it('renders no skip buttons when neither callback is provided', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
        />
      );

      expect(screen.queryByLabelText(/Previous .* message/)).not.toBeInTheDocument();
      expect(screen.queryByLabelText(/Next .* message/)).not.toBeInTheDocument();
    });

    it('calls onSkipToNext when next button is clicked', async () => {
      const user = userEvent.setup();
      const onSkipToNext = vi.fn();
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          roleLabel="User"
          onSkipToNext={onSkipToNext}
        />
      );

      await user.click(screen.getByLabelText('Next User message'));
      expect(onSkipToNext).toHaveBeenCalledOnce();
    });

    it('calls onSkipToPrevious when previous button is clicked', async () => {
      const user = userEvent.setup();
      const onSkipToPrevious = vi.fn();
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          roleLabel="User"
          onSkipToPrevious={onSkipToPrevious}
        />
      );

      await user.click(screen.getByLabelText('Previous User message'));
      expect(onSkipToPrevious).toHaveBeenCalledOnce();
    });

    it('falls back to computed roleLabel when roleLabel prop is not provided', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          onSkipToNext={() => {}}
        />
      );

      // getRoleLabel returns 'User' for a user message
      expect(screen.getByLabelText('Next User message')).toBeInTheDocument();
    });
  });

  // CF-525: approximate per-message output-speed badge. The shared helper
  // (computeMessageTokenSpeed) owns the arithmetic and omission rules; these
  // tests lock the Claude render gate — the badge only shows in cost mode with
  // a real predecessor timestamp, and is omitted when the gap can't yield a
  // rate. Mirrors the Codex coverage in CodexAssistantMessage.test.tsx.
  describe('token speed badge', () => {
    // Fixes output at 1200 tokens; each case sets the timestamp so the gap to
    // previousMessage (10:00:00) drives the rate (e.g. 10:00:02 → 600 tok/s).
    function assistantWithOutput(timestamp: string): AssistantMessage {
      return createAssistantMessage({
        timestamp,
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: { input_tokens: 100, output_tokens: 1200 },
        },
      });
    }

    const previousMessage = createUserMessage({ timestamp: '2025-01-15T10:00:00Z' });

    it('shows the ~tok/s speed badge from the gap to the previous entry', () => {
      render(
        <ClaudeTimelineMessage
          message={assistantWithOutput('2025-01-15T10:00:02Z')}
          previousMessage={previousMessage}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.42}
        />
      );

      expect(screen.getByText('~600 tok/s')).toBeInTheDocument();
    });

    it('omits the speed badge when there is no previous entry', () => {
      render(
        <ClaudeTimelineMessage
          message={assistantWithOutput('2025-01-15T10:00:02Z')}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.42}
        />
      );

      expect(screen.queryByText(/tok\/s/)).not.toBeInTheDocument();
    });

    it('omits the speed badge outside cost mode', () => {
      render(
        <ClaudeTimelineMessage
          message={assistantWithOutput('2025-01-15T10:00:02Z')}
          previousMessage={previousMessage}
          toolNameMap={emptyToolNameMap}
          isCostMode={false}
          messageCost={0.42}
        />
      );

      expect(screen.queryByText(/tok\/s/)).not.toBeInTheDocument();
    });

    it('omits the speed badge when the predecessor shares this timestamp (non-positive gap)', () => {
      render(
        <ClaudeTimelineMessage
          message={assistantWithOutput('2025-01-15T10:00:00Z')}
          previousMessage={previousMessage}
          toolNameMap={emptyToolNameMap}
          isCostMode={true}
          messageCost={0.42}
        />
      );

      expect(screen.queryByText(/tok\/s/)).not.toBeInTheDocument();
    });
  });

  // UNKNOWN rows show the raw JSON payload behind a click-to-expand <details>,
  // matching the convenience Codex/OpenCode rows get from UnknownRawDetails
  // (without reusing that shell — this row has its own header chrome).
  describe('unknown message raw JSON', () => {
    function getRawJsonDetails(): HTMLElement | null {
      return screen.getByText('Raw JSON').closest('details');
    }

    it('renders a collapsed "Raw JSON" details alongside the report button', () => {
      const message = createUnknownMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
        />
      );

      expect(getRawJsonDetails()).not.toHaveAttribute('open');
      expect(screen.getByText('Report bug')).toBeInTheDocument();
    });

    it('shows the raw JSON payload text once expanded', async () => {
      const user = userEvent.setup();
      const message = createUnknownMessage({ fromAgent: 'agent-1', toAgent: 'agent-2' });
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
        />
      );

      await user.click(screen.getByText('Raw JSON'));

      expect(screen.getByText(/"fromAgent"/)).toBeInTheDocument();
      expect(screen.getByText(/"agent-1"/)).toBeInTheDocument();
    });

    it('auto-opens the details when this row is the active search match', () => {
      const message = createUnknownMessage({ fromAgent: 'agent-1' });
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          searchQuery="agent-1"
          isCurrentSearchMatch={true}
        />
      );

      expect(getRawJsonDetails()).toHaveAttribute('open');
    });

    it('does not render the raw JSON details for known message types', () => {
      const message = createUserMessage();
      render(
        <ClaudeTimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
        />
      );

      expect(screen.queryByText('Raw JSON')).not.toBeInTheDocument();
    });
  });

  // et0r: Agent tool results and agent completion notifications render as
  // subagent cards linking to the subagent's own transcript subtab.
  describe('subagent cards', () => {
    const launchLines: TranscriptLine[] = [
      agentToolUse({ uuid: 'u1', toolUseId: 't1', description: 'Explore the codebase', subagentType: 'Explore' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1', description: 'Explore the codebase' }),
    ];

    function withThreads(
      ui: React.ReactElement,
      { onOpenThread, activeThreadId = null, lines = launchLines }: {
        onOpenThread?: (id: string) => void;
        activeThreadId?: string | null;
        lines?: TranscriptLine[];
      } = {},
    ) {
      return render(
        <ClaudeThreadContext.Provider
          value={{ agentIndex: buildClaudeAgentIndex(lines), onOpenThread, activeThreadId }}
        >
          {ui}
        </ClaudeThreadContext.Provider>,
      );
    }

    const toolNameMap = new Map([['t1', 'Agent']]);

    it('renders a launch card in place of the Agent tool_result block', async () => {
      const user = userEvent.setup();
      const onOpenThread = vi.fn();
      withThreads(<ClaudeTimelineMessage message={launchLines[1]!} toolNameMap={toolNameMap} />, { onOpenThread });

      expect(screen.getByText('Explore the codebase')).toBeInTheDocument();
      expect(screen.getByText('Raw result')).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Open transcript →' }));
      expect(onOpenThread).toHaveBeenCalledWith('a1');
    });

    it('renders the Agent tool_result as a plain block when no thread context is provided', () => {
      render(<ClaudeTimelineMessage message={launchLines[1]!} toolNameMap={toolNameMap} />);
      expect(screen.queryByText('Raw result')).not.toBeInTheDocument();
    });

    it('renders a finished card for an agent <task-notification> message', () => {
      const notification = taskNotificationMessage('n1', {
        taskId: 'a1',
        summary: 'Agent "Explore the codebase" finished',
      });
      withThreads(<ClaudeTimelineMessage message={notification} toolNameMap={emptyToolNameMap} />, {
        lines: [...launchLines, notification],
        onOpenThread: () => {},
      });
      expect(screen.getByText('Subagent finished')).toBeInTheDocument();
      expect(screen.getByText('Agent "Explore the codebase" finished')).toBeInTheDocument();
      expect(screen.getByText('Raw notification')).toBeInTheDocument();
    });

    it('leaves background-command task notifications (unknown task id) as plain text', () => {
      const notification = taskNotificationMessage('n1', {
        taskId: 'bash-task',
        toolUseId: 'toolu_bash',
        summary: 'Background command "Build" completed (exit code 0)',
      });
      withThreads(<ClaudeTimelineMessage message={notification} toolNameMap={emptyToolNameMap} />, {
        lines: [...launchLines, notification],
      });
      expect(screen.queryByText('Subagent finished')).not.toBeInTheDocument();
    });

    it('hides the agentId badge inside the agent\'s own thread', () => {
      const line = subagentAssistantText('s1', 'a1', 'working');
      const { unmount } = withThreads(<ClaudeTimelineMessage message={line} toolNameMap={emptyToolNameMap} />, {
        activeThreadId: 'a1',
      });
      expect(screen.queryByText('a1')).not.toBeInTheDocument();
      unmount();

      withThreads(<ClaudeTimelineMessage message={line} toolNameMap={emptyToolNameMap} />, { activeThreadId: null });
      expect(screen.getByText('a1')).toBeInTheDocument();
    });

    it('copy-link includes the agent param inside a subagent thread', async () => {
      const user = userEvent.setup();
      const line = subagentAssistantText('s1', 'a1', 'working');
      withThreads(
        <ClaudeTimelineMessage message={line} toolNameMap={emptyToolNameMap} sessionId="session-abc" />,
        { activeThreadId: 'a1' },
      );
      await user.click(screen.getByLabelText('Copy link to message'));
      expect(copiedTexts[0]).toMatch(/\?tab=transcript&msg=s1&agent=a1$/);
    });
  });
});
