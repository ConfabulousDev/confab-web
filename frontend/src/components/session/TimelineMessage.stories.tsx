import type { Meta, StoryObj } from '@storybook/react-vite';
import type { UserMessage, AssistantMessage, TranscriptLine } from '@/types';
import type { SystemMessage } from '@/schemas/transcript';
import TimelineMessage from './TimelineMessage';

const emptyToolNameMap = new Map<string, string>();

const mockUserMessage: UserMessage = {
  type: 'user',
  uuid: 'user-uuid-1',
  timestamp: '2025-01-15T10:00:00Z',
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/project',
  sessionId: 'session-123',
  version: '1.0.0',
  message: {
    role: 'user',
    content: 'Help me build an analytics feature for tracking session metrics',
  },
};

const mockAssistantMessage: AssistantMessage = {
  type: 'assistant',
  uuid: 'assistant-uuid-1',
  timestamp: '2025-01-15T10:00:05Z',
  parentUuid: 'user-uuid-1',
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/project',
  sessionId: 'session-123',
  version: '1.0.0',
  requestId: 'req-1',
  message: {
    model: 'claude-sonnet-4-20250514',
    id: 'msg-1',
    type: 'message',
    role: 'assistant',
    content: [
      {
        type: 'text',
        text: "I'll help you build an analytics feature. Let me start by exploring your codebase to understand the current structure.\n\nFirst, I'll look at your existing data models and API endpoints.",
      },
    ],
    stop_reason: 'end_turn',
    stop_sequence: null,
    usage: {
      input_tokens: 15000,
      output_tokens: 2500,
      cache_creation_input_tokens: 5000,
      cache_read_input_tokens: 0,
    },
  },
};

const mockSystemMessage: SystemMessage = {
  type: 'system',
  uuid: 'system-uuid-1',
  timestamp: '2025-01-15T10:00:10Z',
  parentUuid: 'assistant-uuid-1',
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/project',
  sessionId: 'session-123',
  version: '1.0.0',
  subtype: 'info',
  content: 'Session context loaded successfully',
};

const mockFileSnapshot: TranscriptLine = {
  type: 'file-history-snapshot',
  messageId: 'snap-1',
  isSnapshotUpdate: false,
  snapshot: {
    messageId: 'snap-1',
    timestamp: '2025-01-15T10:00:00Z',
    trackedFileBackups: {
      'src/analytics.ts': { backupFileName: 'analytics.ts.bak', version: 1, backupTime: '2025-01-15T10:00:00Z' },
    },
  },
};

const meta = {
  title: 'Session/TimelineMessage',
  component: TimelineMessage,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 800 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof TimelineMessage>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default user message with copy-link button visible on hover.
 * Hover to see both the copy and link buttons appear.
 */
export const UserMessageStory: Story = {
  name: 'User Message',
  args: {
    message: mockUserMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
  },
};

/**
 * Assistant message with token count and model badge.
 */
export const AssistantMessageStory: Story = {
  name: 'Assistant Message',
  args: {
    message: mockAssistantMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
  },
};

/**
 * Message highlighted as a deep-link target.
 * Shows the persistent accent border with initial pulse animation.
 */
export const DeepLinkTarget: Story = {
  args: {
    message: mockAssistantMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isDeepLinkTarget: true,
  },
};

/**
 * Both selected (hover/seek) and deep-link target active simultaneously.
 * The accent border from deep-link takes visual priority.
 */
export const SelectedAndDeepLinkTarget: Story = {
  args: {
    message: mockAssistantMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isSelected: true,
    isDeepLinkTarget: true,
  },
};

/**
 * Selected message without deep-link (normal hover/seek state).
 * Shows the grey selection border.
 */
export const SelectedOnly: Story = {
  args: {
    message: mockUserMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isSelected: true,
  },
};

/**
 * System message as deep-link target.
 */
export const SystemDeepLinkTarget: Story = {
  args: {
    message: mockSystemMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isDeepLinkTarget: true,
  },
};

/**
 * File history snapshot — no copy-link button since it has no uuid.
 */
export const FileSnapshotNoLinkButton: Story = {
  args: {
    message: mockFileSnapshot,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
  },
};

/**
 * Message without sessionId — no copy-link button rendered.
 */
export const NoSessionId: Story = {
  args: {
    message: mockUserMessage,
    toolNameMap: emptyToolNameMap,
  },
};

/**
 * User message with both skip navigation buttons.
 * Hover to see ↑ and ↓ arrows for jumping to previous/next User message.
 */
export const WithSkipBothDirections: Story = {
  name: 'Skip Navigation (Both)',
  args: {
    message: mockUserMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    roleLabel: 'User',
    onSkipToNext: () => {},
    onSkipToPrevious: () => {},
  },
};

/**
 * First message of its type — only "next" skip button, no "previous".
 */
export const WithSkipNextOnly: Story = {
  name: 'Skip Navigation (Next Only)',
  args: {
    message: mockAssistantMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    roleLabel: 'Assistant',
    onSkipToNext: () => {},
  },
};

/**
 * Last message of its type — only "previous" skip button, no "next".
 */
export const WithSkipPreviousOnly: Story = {
  name: 'Skip Navigation (Previous Only)',
  args: {
    message: mockAssistantMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    roleLabel: 'Assistant',
    onSkipToPrevious: () => {},
  },
};
