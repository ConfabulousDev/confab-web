import type { Meta, StoryObj } from '@storybook/react-vite';
import type { SessionAnalytics } from '@/services/api';
import SessionAnalyticsPanel from './SessionAnalyticsPanel';

// Sample analytics from backend API - typical session
const mockAnalytics: SessionAnalytics = {
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
};

// Empty analytics (new session, no activity)
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

// Small session analytics
const smallAnalytics: SessionAnalytics = {
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
};

// Large session with heavy usage
const largeAnalytics: SessionAnalytics = {
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
};

// Only auto compactions
const autoCompactionAnalytics: SessionAnalytics = {
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
};

// Analytics computed a while ago
const olderAnalytics: SessionAnalytics = {
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
 * Default view - analytics display with "Updated X ago" timestamp.
 * Analytics are polled automatically when tab is visible.
 */
export const Default: Story = {
  args: {
    sessionId: 'test-session-id',
    initialAnalytics: mockAnalytics,
  },
};

/**
 * Analytics computed 1 hour ago.
 * Shows "Updated 1 hour ago" timestamp.
 */
export const OlderTimestamp: Story = {
  args: {
    sessionId: 'test-session-id',
    initialAnalytics: olderAnalytics,
  },
};

/**
 * Empty session with no messages yet.
 */
export const EmptySession: Story = {
  args: {
    sessionId: 'test-session-id',
    initialAnalytics: emptyAnalytics,
  },
};

/**
 * Small session with minimal activity.
 */
export const SmallSession: Story = {
  args: {
    sessionId: 'test-session-id',
    initialAnalytics: smallAnalytics,
  },
};

/**
 * Large session with heavy usage.
 */
export const LargeSession: Story = {
  args: {
    sessionId: 'test-session-id',
    initialAnalytics: largeAnalytics,
  },
};

/**
 * Session with only auto compactions (no manual).
 */
export const AutoCompactionsOnly: Story = {
  args: {
    sessionId: 'test-session-id',
    initialAnalytics: autoCompactionAnalytics,
  },
};

// Note: Loading and Error states require mocking the API
// These can be added with MSW (Mock Service Worker) if needed
