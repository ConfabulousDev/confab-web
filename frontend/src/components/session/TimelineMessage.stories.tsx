import type { Meta, StoryObj } from '@storybook/react-vite';
import type { UserMessage, AssistantMessage, SystemMessage, TranscriptLine } from '@/types';
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

const mockAssistantWithWebSearch: AssistantMessage = {
  ...mockAssistantMessage,
  uuid: 'assistant-ws-1',
  message: {
    ...mockAssistantMessage.message,
    usage: {
      ...mockAssistantMessage.message.usage,
      server_tool_use: {
        web_search_requests: 3,
        web_fetch_requests: 1,
      },
    },
  },
};

/**
 * Assistant message with server tool usage badges (web search + fetch).
 */
export const WithWebSearch: Story = {
  name: 'Web Search + Fetch',
  args: {
    message: mockAssistantWithWebSearch,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
  },
};

const mockAssistantFastMode: AssistantMessage = {
  ...mockAssistantMessage,
  uuid: 'assistant-fast-1',
  message: {
    ...mockAssistantMessage.message,
    model: 'claude-opus-4-6-20260201',
    usage: {
      ...mockAssistantMessage.message.usage,
      speed: 'fast',
    },
  },
};

/**
 * Assistant message in fast mode — shows lightning bolt badge.
 */
export const FastMode: Story = {
  name: 'Fast Mode',
  args: {
    message: mockAssistantFastMode,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
  },
};

/**
 * Cost mode — shows red cost badge instead of token count.
 */
export const CostMode: Story = {
  name: 'Cost Mode',
  args: {
    message: mockAssistantMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isCostMode: true,
    messageCost: 0.42,
  },
};

/**
 * Cost mode with an expensive message (high-cost opus).
 */
export const CostModeExpensive: Story = {
  name: 'Cost Mode (Expensive)',
  args: {
    message: {
      ...mockAssistantMessage,
      uuid: 'expensive-msg',
      message: {
        ...mockAssistantMessage.message,
        model: 'claude-opus-4-5-20251101',
        usage: {
          input_tokens: 150000,
          output_tokens: 25000,
          cache_creation_input_tokens: 50000,
          cache_read_input_tokens: 80000,
        },
      },
    },
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isCostMode: true,
    messageCost: 4.73,
  },
};

/**
 * Cost mode with cache read only (common for follow-up turns that hit warm cache).
 */
export const CostModeCacheReadOnly: Story = {
  name: 'Cost Mode (Cache Read Only)',
  args: {
    message: {
      ...mockAssistantMessage,
      uuid: 'cache-read-msg',
      message: {
        ...mockAssistantMessage.message,
        usage: {
          input_tokens: 12000,
          output_tokens: 800,
          cache_creation_input_tokens: 0,
          cache_read_input_tokens: 45000,
        },
      },
    },
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isCostMode: true,
    messageCost: 0.08,
  },
};

/**
 * Cost mode on a user message (no cost badge shown — users have no token usage).
 */
export const CostModeUserMessage: Story = {
  name: 'Cost Mode (User)',
  args: {
    message: mockUserMessage,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
    isCostMode: true,
    // messageCost is undefined for user messages
  },
};

const mockAssistantFastWithSearch: AssistantMessage = {
  ...mockAssistantMessage,
  uuid: 'assistant-fast-ws-1',
  message: {
    ...mockAssistantMessage.message,
    model: 'claude-opus-4-6-20260201',
    usage: {
      ...mockAssistantMessage.message.usage,
      speed: 'fast',
      server_tool_use: {
        web_search_requests: 5,
        web_fetch_requests: 2,
        code_execution_requests: 1,
      },
    },
  },
};

/**
 * Fast mode with all server tool types — shows lightning + search + fetch + exec badges.
 */
export const FastModeWithAllServerTools: Story = {
  name: 'Fast Mode + All Server Tools',
  args: {
    message: mockAssistantFastWithSearch,
    toolNameMap: emptyToolNameMap,
    sessionId: 'test-session-id',
  },
};
