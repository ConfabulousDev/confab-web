import type { Meta, StoryObj } from '@storybook/react-vite';
import type { Session } from '@/types';
import SessionListStatsSidebar from './SessionListStatsSidebar';

// Helper to create mock sessions
function createMockSession(overrides: Partial<Session> = {}): Session {
  const now = new Date();
  const firstSeen = new Date(now.getTime() - Math.random() * 30 * 24 * 60 * 60 * 1000); // Random time in last 30 days
  const lastSync = new Date(firstSeen.getTime() + Math.random() * 4 * 60 * 60 * 1000); // 0-4 hours after first seen

  return {
    id: Math.random().toString(36).substring(7),
    external_id: Math.random().toString(36).substring(2, 10),
    first_seen: firstSeen.toISOString(),
    last_sync_time: lastSync.toISOString(),
    file_count: Math.floor(Math.random() * 5) + 1,
    summary: 'Mock session summary',
    first_user_message: 'What is the meaning of life?',
    session_type: 'claude-code',
    total_lines: Math.floor(Math.random() * 500) + 50,
    git_repo: 'my-project',
    git_branch: 'main',
    is_owner: true,
    access_type: 'owner',
    shared_by_email: null,
    ...overrides,
  };
}

// Generate mock sessions with varying dates and durations
function generateMockSessions(count: number): Session[] {
  const sessions: Session[] = [];
  const now = new Date();

  for (let i = 0; i < count; i++) {
    // Spread sessions over the last 30 days
    const daysAgo = Math.random() * 30;
    const firstSeen = new Date(now.getTime() - daysAgo * 24 * 60 * 60 * 1000);

    // Random duration: 5 minutes to 6 hours
    const durationMs = (5 + Math.random() * 355) * 60 * 1000;
    const lastSync = new Date(firstSeen.getTime() + durationMs);

    sessions.push(
      createMockSession({
        first_seen: firstSeen.toISOString(),
        last_sync_time: lastSync.toISOString(),
      })
    );
  }

  return sessions;
}

const meta: Meta<typeof SessionListStatsSidebar> = {
  title: 'Components/SessionListStatsSidebar',
  component: SessionListStatsSidebar,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '220px', height: '400px', background: '#fafafa' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof SessionListStatsSidebar>;

export const Default: Story = {
  args: {
    sessions: generateMockSessions(25),
    loading: false,
  },
};

export const Loading: Story = {
  args: {
    sessions: [],
    loading: true,
  },
};

export const NoSessions: Story = {
  args: {
    sessions: [],
    loading: false,
  },
};

export const SingleSession: Story = {
  args: {
    sessions: [createMockSession()],
    loading: false,
  },
};

export const ManySessions: Story = {
  args: {
    sessions: generateMockSessions(100),
    loading: false,
  },
};

// Sessions with very short durations
export const ShortDurations: Story = {
  args: {
    sessions: Array.from({ length: 20 }, () => {
      const now = new Date();
      const firstSeen = new Date(now.getTime() - Math.random() * 7 * 24 * 60 * 60 * 1000);
      const durationMs = (1 + Math.random() * 10) * 60 * 1000; // 1-10 minutes
      return createMockSession({
        first_seen: firstSeen.toISOString(),
        last_sync_time: new Date(firstSeen.getTime() + durationMs).toISOString(),
      });
    }),
    loading: false,
  },
};

// Sessions with very long durations
export const LongDurations: Story = {
  args: {
    sessions: Array.from({ length: 20 }, () => {
      const now = new Date();
      const firstSeen = new Date(now.getTime() - Math.random() * 14 * 24 * 60 * 60 * 1000);
      const durationMs = (2 + Math.random() * 22) * 60 * 60 * 1000; // 2-24 hours
      return createMockSession({
        first_seen: firstSeen.toISOString(),
        last_sync_time: new Date(firstSeen.getTime() + durationMs).toISOString(),
      });
    }),
    loading: false,
  },
};

// Sessions created recently (high recent activity)
export const HighRecentActivity: Story = {
  args: {
    sessions: Array.from({ length: 50 }, () => {
      const now = new Date();
      const firstSeen = new Date(now.getTime() - Math.random() * 5 * 24 * 60 * 60 * 1000); // All in last 5 days
      const durationMs = Math.random() * 2 * 60 * 60 * 1000;
      return createMockSession({
        first_seen: firstSeen.toISOString(),
        last_sync_time: new Date(firstSeen.getTime() + durationMs).toISOString(),
      });
    }),
    loading: false,
  },
};

// Sessions spread over time (low recent activity)
export const LowRecentActivity: Story = {
  args: {
    sessions: Array.from({ length: 50 }, () => {
      const now = new Date();
      // Most sessions are older than 30 days
      const daysAgo = 30 + Math.random() * 60;
      const firstSeen = new Date(now.getTime() - daysAgo * 24 * 60 * 60 * 1000);
      const durationMs = Math.random() * 2 * 60 * 60 * 1000;
      return createMockSession({
        first_seen: firstSeen.toISOString(),
        last_sync_time: new Date(firstSeen.getTime() + durationMs).toISOString(),
      });
    }),
    loading: false,
  },
};

// Sessions without last_sync_time (no duration data)
export const NoDurationData: Story = {
  args: {
    sessions: Array.from({ length: 15 }, () =>
      createMockSession({
        last_sync_time: null,
      })
    ),
    loading: false,
  },
};
