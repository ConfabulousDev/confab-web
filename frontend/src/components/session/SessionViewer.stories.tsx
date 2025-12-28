import type { Meta, StoryObj } from '@storybook/react-vite';
import type { SessionDetail, AssistantMessage, UserMessage, TranscriptLine } from '@/types';
import type { SessionAnalytics, GitHubLink } from '@/services/api';
import { KeyboardShortcutProvider } from '@/contexts/KeyboardShortcutContext';
import SessionViewer from './SessionViewer';

// Mock session detail
const mockSession: SessionDetail = {
  id: 'test-session-uuid',
  external_id: 'abc123def456',
  custom_title: null,
  summary: 'Building a session analytics feature',
  first_user_message: 'Help me build analytics for my app',
  first_seen: '2025-01-15T10:00:00Z',
  last_sync_at: '2025-01-15T12:30:00Z',
  cwd: '/Users/dev/my-project',
  transcript_path: '/Users/dev/.claude/sessions/abc123/transcript.jsonl',
  git_info: {
    repo_url: 'https://github.com/user/repo',
    branch: 'feature/analytics',
    commit_sha: 'abc1234',
    commit_message: 'Add analytics endpoint',
    author: 'Developer',
    is_dirty: false,
  },
  files: [
    {
      file_name: 'transcript.jsonl',
      file_type: 'transcript',
      last_synced_line: 10,
      updated_at: '2025-01-15T12:30:00Z',
    },
  ],
  hostname: 'dev-machine',
  username: 'developer',
  is_owner: true,
};

// Mock transcript messages
const mockUserMessage: UserMessage = {
  type: 'user',
  uuid: 'msg-1',
  timestamp: '2025-01-15T10:00:00Z',
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/my-project',
  sessionId: 'abc123def456',
  version: '1.0.0',
  message: {
    role: 'user',
    content: 'Help me build an analytics feature for tracking session metrics',
  },
};

const mockAssistantMessage: AssistantMessage = {
  type: 'assistant',
  uuid: 'msg-2',
  timestamp: '2025-01-15T10:00:05Z',
  parentUuid: 'msg-1',
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/my-project',
  sessionId: 'abc123def456',
  version: '1.0.0',
  requestId: 'req-1',
  message: {
    model: 'claude-sonnet-4-20250514',
    id: 'msg-assistant-1',
    type: 'message',
    role: 'assistant',
    content: [
      {
        type: 'text',
        text: "I'll help you build an analytics feature. Let me start by exploring your codebase to understand the current structure.\n\nFirst, I'll look at your existing data models and API endpoints to see how we can integrate analytics tracking.",
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

const mockUserMessage2: UserMessage = {
  type: 'user',
  uuid: 'msg-3',
  timestamp: '2025-01-15T10:01:00Z',
  parentUuid: 'msg-2',
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/my-project',
  sessionId: 'abc123def456',
  version: '1.0.0',
  message: {
    role: 'user',
    content: 'Great, please focus on token usage and cost tracking',
  },
};

const mockAssistantMessage2: AssistantMessage = {
  type: 'assistant',
  uuid: 'msg-4',
  timestamp: '2025-01-15T10:01:10Z',
  parentUuid: 'msg-3',
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/my-project',
  sessionId: 'abc123def456',
  version: '1.0.0',
  requestId: 'req-2',
  message: {
    model: 'claude-sonnet-4-20250514',
    id: 'msg-assistant-2',
    type: 'message',
    role: 'assistant',
    content: [
      {
        type: 'text',
        text: "Perfect, I'll create a comprehensive token tracking system. Here's my plan:\n\n1. Parse JSONL transcripts to extract usage data from assistant messages\n2. Calculate costs based on model-specific pricing\n3. Track cache efficiency (creation vs read tokens)\n4. Store computed analytics in a database cache\n\nLet me start implementing this...",
      },
    ],
    stop_reason: 'end_turn',
    stop_sequence: null,
    usage: {
      input_tokens: 18000,
      output_tokens: 3200,
      cache_creation_input_tokens: 0,
      cache_read_input_tokens: 5000,
    },
  },
};

const mockMessages: TranscriptLine[] = [
  mockUserMessage,
  mockAssistantMessage,
  mockUserMessage2,
  mockAssistantMessage2,
];

// Mock analytics computed from the messages above
// computed_lines matches mockSession.files[0].last_synced_line (10)
const mockAnalytics: SessionAnalytics = {
  computed_at: new Date(Date.now() - 60000).toISOString(), // 1 minute ago
  computed_lines: 10,
  tokens: {
    input: 33000,
    output: 5700,
    cache_creation: 5000,
    cache_read: 5000,
  },
  cost: {
    estimated_usd: '1.45',
  },
  compaction: {
    auto: 0,
    manual: 0,
    avg_time_ms: null,
  },
};

// Mock GitHub links
const mockGithubLinks: GitHubLink[] = [
  {
    id: 1,
    session_id: 'test-session-uuid',
    link_type: 'pull_request',
    url: 'https://github.com/user/repo/pull/42',
    owner: 'user',
    repo: 'repo',
    ref: '42',
    title: 'Add analytics feature',
    source: 'cli_hook',
    created_at: '2025-01-15T11:00:00Z',
  },
];

const meta = {
  title: 'Session/SessionViewer',
  component: SessionViewer,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <KeyboardShortcutProvider>
        <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
          <Story />
        </div>
      </KeyboardShortcutProvider>
    ),
  ],
} satisfies Meta<typeof SessionViewer>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default view with Summary tab active.
 * Click the "Transcript" tab to switch views.
 */
export const Default: Story = {
  args: {
    session: mockSession,
    isOwner: true,
    isShared: false,
    initialMessages: mockMessages,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: mockGithubLinks,
  },
};

/**
 * Shared session view (non-owner).
 * Share and Delete buttons are hidden.
 */
export const SharedSession: Story = {
  args: {
    session: {
      ...mockSession,
      is_owner: false,
      hostname: null,
      username: null,
    },
    isOwner: false,
    isShared: true,
    initialMessages: mockMessages,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: mockGithubLinks,
  },
};

/**
 * Session with a custom title set by the user.
 */
export const WithCustomTitle: Story = {
  args: {
    session: {
      ...mockSession,
      custom_title: 'Analytics Implementation Session',
    },
    isOwner: true,
    isShared: false,
    initialMessages: mockMessages,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: mockGithubLinks,
  },
};

/**
 * Empty session with no messages yet.
 */
// Empty analytics for new sessions
const emptyAnalytics: SessionAnalytics = {
  computed_at: new Date().toISOString(),
  computed_lines: 0,
  tokens: {
    input: 0,
    output: 0,
    cache_creation: 0,
    cache_read: 0,
  },
  cost: {
    estimated_usd: '0.00',
  },
  compaction: {
    auto: 0,
    manual: 0,
    avg_time_ms: null,
  },
};

export const EmptySession: Story = {
  args: {
    session: {
      ...mockSession,
      files: [],
    },
    isOwner: true,
    isShared: false,
    initialMessages: [],
    initialAnalytics: emptyAnalytics,
    initialGithubLinks: [],
  },
};
