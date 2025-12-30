import type { Meta, StoryObj } from '@storybook/react-vite';
import type { SessionAnalytics, GitHubLink } from '@/schemas/api';
import SessionSummaryPanel from './SessionSummaryPanel';

// Helper to create analytics with both legacy and cards format
function createAnalytics(base: {
  computed_at: string;
  computed_lines: number;
  tokens: { input: number; output: number; cache_creation: number; cache_read: number };
  cost: { estimated_usd: string };
  compaction: { auto: number; manual: number; avg_time_ms: number | null };
  session?: {
    total_messages?: number;
    user_messages?: number;
    assistant_messages?: number;
    human_prompts?: number;
    tool_results?: number;
    text_responses?: number;
    tool_calls?: number;
    thinking_blocks?: number;
    user_turns: number;
    assistant_turns: number;
    duration_ms: number | null;
    models_used: string[];
  };
}): SessionAnalytics {
  const userTurns = base.session?.user_turns ?? 10;
  const assistantTurns = base.session?.assistant_turns ?? 10;
  // Default message breakdown assumes moderate tool usage
  const totalMessages = base.session?.total_messages ?? (userTurns + assistantTurns) * 5;
  const userMessages = base.session?.user_messages ?? userTurns * 3;
  const assistantMessages = base.session?.assistant_messages ?? totalMessages - userMessages;

  return {
    ...base,
    cards: {
      tokens: {
        ...base.tokens,
        estimated_usd: base.cost.estimated_usd,
      },
      session: {
        // Message counts
        total_messages: totalMessages,
        user_messages: userMessages,
        assistant_messages: assistantMessages,
        // Message type breakdown
        human_prompts: base.session?.human_prompts ?? userTurns,
        tool_results: base.session?.tool_results ?? userMessages - userTurns,
        text_responses: base.session?.text_responses ?? assistantTurns,
        tool_calls: base.session?.tool_calls ?? Math.floor(assistantMessages * 0.6),
        thinking_blocks: base.session?.thinking_blocks ?? Math.floor(assistantMessages * 0.3),
        // Turns
        user_turns: userTurns,
        assistant_turns: assistantTurns,
        // Metadata
        duration_ms: base.session?.duration_ms ?? 3600000,
        models_used: base.session?.models_used ?? ['claude-sonnet-4-20250514'],
        compaction_auto: base.compaction.auto,
        compaction_manual: base.compaction.manual,
        compaction_avg_time_ms: base.compaction.avg_time_ms,
      },
    },
  };
}

// Sample analytics from backend API - typical session
const mockAnalytics = createAnalytics({
  computed_at: new Date(Date.now() - 120000).toISOString(), // 2 minutes ago
  computed_lines: 500,
  tokens: {
    input: 110000,
    output: 20800,
    cache_creation: 23000,
    cache_read: 36000,
  },
  cost: {
    estimated_usd: '4.23',
  },
  compaction: {
    auto: 2,
    manual: 1,
    avg_time_ms: 48500, // 48.5 seconds
  },
});

// Empty analytics (new session, no activity)
const emptyAnalytics = createAnalytics({
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
  session: {
    user_turns: 0,
    assistant_turns: 0,
    duration_ms: null,
    models_used: [],
  },
});

// Small session analytics
const smallAnalytics = createAnalytics({
  computed_at: new Date(Date.now() - 60000).toISOString(), // 1 minute ago
  computed_lines: 25,
  tokens: {
    input: 1100,
    output: 350,
    cache_creation: 200,
    cache_read: 200,
  },
  cost: {
    estimated_usd: '0.05',
  },
  compaction: {
    auto: 0,
    manual: 0,
    avg_time_ms: null,
  },
  session: {
    user_turns: 3,
    assistant_turns: 3,
    duration_ms: 180000,
    models_used: ['claude-sonnet-4-20250514'],
  },
});

// Large session with heavy usage
const largeAnalytics = createAnalytics({
  computed_at: new Date(Date.now() - 300000).toISOString(), // 5 minutes ago
  computed_lines: 2500,
  tokens: {
    input: 2500000,
    output: 450000,
    cache_creation: 150000,
    cache_read: 2000000,
  },
  cost: {
    estimated_usd: '127.45',
  },
  compaction: {
    auto: 15,
    manual: 3,
    avg_time_ms: 52300,
  },
  session: {
    user_turns: 50,
    assistant_turns: 50,
    duration_ms: 14400000,
    models_used: ['claude-sonnet-4-20250514', 'claude-opus-4-5-20251101'],
  },
});

// Only auto compactions
const autoCompactionAnalytics = createAnalytics({
  computed_at: new Date(Date.now() - 180000).toISOString(), // 3 minutes ago
  computed_lines: 800,
  tokens: {
    input: 500000,
    output: 85000,
    cache_creation: 50000,
    cache_read: 400000,
  },
  cost: {
    estimated_usd: '28.50',
  },
  compaction: {
    auto: 5,
    manual: 0,
    avg_time_ms: 45000,
  },
});

// Analytics computed a while ago
const olderAnalytics = createAnalytics({
  computed_at: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
  computed_lines: 450,
  tokens: {
    input: 95000,
    output: 18000,
    cache_creation: 20000,
    cache_read: 30000,
  },
  cost: {
    estimated_usd: '3.85',
  },
  compaction: {
    auto: 2,
    manual: 0,
    avg_time_ms: 42000,
  },
});

// Sample GitHub links for stories
const mockGitHubLinks: GitHubLink[] = [
  {
    id: 1,
    session_id: 'test-session',
    link_type: 'pull_request',
    url: 'https://github.com/owner/repo/pull/123',
    owner: 'owner',
    repo: 'repo',
    ref: '123',
    title: 'Add new feature',
    source: 'cli_hook',
    created_at: '2025-01-15T10:30:00Z',
  },
  {
    id: 2,
    session_id: 'test-session',
    link_type: 'pull_request',
    url: 'https://github.com/another-org/another-repo/pull/456',
    owner: 'another-org',
    repo: 'another-repo',
    ref: '456',
    title: 'Fix critical bug',
    source: 'manual',
    created_at: '2025-01-15T09:00:00Z',
  },
];

const mockCommitLinks: GitHubLink[] = [
  {
    id: 3,
    session_id: 'test-session',
    link_type: 'commit',
    url: 'https://github.com/owner/repo/commit/abc1234567890def',
    owner: 'owner',
    repo: 'repo',
    ref: 'abc1234567890def',
    title: null,
    source: 'cli_hook',
    created_at: '2025-01-15T11:00:00Z',
  },
  {
    id: 4,
    session_id: 'test-session',
    link_type: 'commit',
    url: 'https://github.com/owner/repo/commit/def4567890abcdef',
    owner: 'owner',
    repo: 'repo',
    ref: 'def4567890abcdef',
    title: null,
    source: 'cli_hook',
    created_at: '2025-01-15T10:30:00Z',
  },
  {
    id: 5,
    session_id: 'test-session',
    link_type: 'commit',
    url: 'https://github.com/owner/repo/commit/789abcdef0123456',
    owner: 'owner',
    repo: 'repo',
    ref: '789abcdef0123456',
    title: null,
    source: 'cli_hook',
    created_at: '2025-01-15T10:00:00Z',
  },
];

const meta = {
  title: 'Session/SessionSummaryPanel',
  component: SessionSummaryPanel,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '800px', height: '600px', background: 'var(--color-bg)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SessionSummaryPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default view for owners with no GitHub links.
 * GitHub card is hidden by default. Use the "..." menu to reveal it.
 * Analytics are polled automatically when tab is visible.
 */
export const Default: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: [],
  },
};

/**
 * Analytics computed 1 hour ago.
 * Shows "Updated 1 hour ago" timestamp.
 */
export const OlderTimestamp: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: olderAnalytics,
    initialGithubLinks: [],
  },
};

/**
 * Empty session with no messages yet.
 */
export const EmptySession: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: emptyAnalytics,
    initialGithubLinks: [],
  },
};

/**
 * Small session with minimal activity.
 */
export const SmallSession: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: smallAnalytics,
    initialGithubLinks: [],
  },
};

/**
 * Large session with heavy usage.
 */
export const LargeSession: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: largeAnalytics,
    initialGithubLinks: [],
  },
};

/**
 * Session with only auto compactions (no manual).
 */
export const AutoCompactionsOnly: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: autoCompactionAnalytics,
    initialGithubLinks: [],
  },
};

/**
 * Summary with GitHub PR links.
 */
export const WithGitHubLinks: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: mockGitHubLinks,
  },
};

/**
 * Summary with GitHub PR and commit links.
 * Shows all commits (latest first) with individual delete buttons.
 */
export const WithPRsAndCommits: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: true,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: [...mockGitHubLinks, ...mockCommitLinks],
  },
};

/**
 * View-only mode (non-owner) with GitHub links.
 * No actions menu shown. GitHub card visible because there are links.
 */
export const ViewOnly: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: false,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: mockGitHubLinks,
  },
};

/**
 * View-only mode (non-owner) with no GitHub links.
 * No actions menu and no GitHub card.
 */
export const ViewOnlyNoGitHub: Story = {
  args: {
    sessionId: 'test-session-id',
    isOwner: false,
    initialAnalytics: mockAnalytics,
    initialGithubLinks: [],
  },
};
