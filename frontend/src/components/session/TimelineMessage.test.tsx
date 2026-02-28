import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserMessage, AssistantMessage, TranscriptLine } from '@/types';
import TimelineMessage from './TimelineMessage';

// Mock the useCopyToClipboard hook to capture copied text
const copiedTexts: string[] = [];
vi.mock('@/hooks', () => ({
  useCopyToClipboard: () => ({
    copy: (text: string) => {
      copiedTexts.push(text);
      return Promise.resolve();
    },
    copied: copiedTexts.length > 0,
    message: null,
  }),
}));

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

describe('TimelineMessage', () => {
  beforeEach(() => {
    copiedTexts.length = 0;
  });

  describe('copy-link button', () => {
    it('renders link button for messages with uuid and sessionId', () => {
      const message = createUserMessage();
      render(
        <TimelineMessage
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
        <TimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
        />
      );

      expect(screen.queryByLabelText('Copy link to message')).not.toBeInTheDocument();
    });

    it('does not render link button for messages without uuid', () => {
      const message = createFileHistorySnapshot();
      render(
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
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
        <TimelineMessage
          message={message}
          toolNameMap={emptyToolNameMap}
          onSkipToNext={() => {}}
        />
      );

      // getRoleLabel returns 'User' for a user message
      expect(screen.getByLabelText('Next User message')).toBeInTheDocument();
    });
  });
});
