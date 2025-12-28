import type { Meta, StoryObj } from '@storybook/react-vite';
import type { SessionAnalytics } from '@/services/api';
import SessionAnalyticsPanel from './SessionAnalyticsPanel';

// Sample analytics from backend API - typical session (up to date)
const mockAnalytics: SessionAnalytics = {
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
};

// Empty analytics (new session, no activity)
const emptyAnalytics: SessionAnalytics = {
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

// Small session analytics
const smallAnalytics: SessionAnalytics = {
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
};

// Large session with heavy usage
const largeAnalytics: SessionAnalytics = {
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
};

// Only auto compactions
const autoCompactionAnalytics: SessionAnalytics = {
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
};

// Stale analytics (computed from fewer lines than current)
const staleAnalytics: SessionAnalytics = {
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
};

const meta = {
  title: 'Session/SessionAnalyticsPanel',
  component: SessionAnalyticsPanel,
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
} satisfies Meta<typeof SessionAnalyticsPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default view - analytics are up to date (totalLines matches computed_lines).
 * Refresh button shows "Up to date" and is disabled.
 */
export const Default: Story = {
  args: {
    sessionId: 'test-session-id',
    totalLines: 500, // Matches computed_lines
    initialAnalytics: mockAnalytics,
  },
};

/**
 * Stale analytics - new lines available since last computation.
 * Refresh button shows "+50 lines" and is enabled.
 */
export const Stale: Story = {
  args: {
    sessionId: 'test-session-id',
    totalLines: 500, // 50 more than computed_lines (450)
    initialAnalytics: staleAnalytics,
  },
};

/**
 * Empty session with no messages yet.
 */
export const EmptySession: Story = {
  args: {
    sessionId: 'test-session-id',
    totalLines: 0,
    initialAnalytics: emptyAnalytics,
  },
};

/**
 * Small session with minimal activity.
 */
export const SmallSession: Story = {
  args: {
    sessionId: 'test-session-id',
    totalLines: 25,
    initialAnalytics: smallAnalytics,
  },
};

/**
 * Large session with heavy usage.
 */
export const LargeSession: Story = {
  args: {
    sessionId: 'test-session-id',
    totalLines: 2500,
    initialAnalytics: largeAnalytics,
  },
};

/**
 * Session with only auto compactions (no manual).
 */
export const AutoCompactionsOnly: Story = {
  args: {
    sessionId: 'test-session-id',
    totalLines: 800,
    initialAnalytics: autoCompactionAnalytics,
  },
};

// Note: Loading and Error states require mocking the API
// These can be added with MSW (Mock Service Worker) if needed
