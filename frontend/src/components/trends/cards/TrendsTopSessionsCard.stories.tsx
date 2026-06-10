import type { Meta, StoryObj } from '@storybook/react-vite';
import { PROVIDER_VALUES, type ProviderId } from '@/utils/providers';
import { TrendsTopSessionsCard } from './TrendsTopSessionsCard';

const meta: Meta<typeof TrendsTopSessionsCard> = {
  title: 'Trends/Cards/TrendsTopSessionsCard',
  component: TrendsTopSessionsCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '700px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TrendsTopSessionsCard>;

// Mixed-provider rows cycle through `PROVIDER_VALUES` so adding a third
// provider extends the fixture automatically. The card under test displays
// per-provider icons; the cycle guarantees every provider appears in the
// preview. Specific rows (`UntitledRow` below) keep an explicit literal
// because they exercise the unknown-provider fallback path.
function rotateProvider(idx: number): ProviderId {
  return PROVIDER_VALUES[idx % PROVIDER_VALUES.length]!;
}

export const Default: Story = {
  args: {
    data: {
      sessions: [
        {
          id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
          title: 'Implement comprehensive dark mode with theme system',
          provider: rotateProvider(0),
          estimated_cost_usd: '45.20',
          duration_ms: 7200000,
          git_repo: 'org/frontend-app',
        },
        {
          id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
          title: 'Debug OAuth login redirect loop in production',
          provider: rotateProvider(1),
          estimated_cost_usd: '32.15',
          duration_ms: 5400000,
          git_repo: 'org/auth-service',
        },
        {
          id: 'c3d4e5f6-a7b8-9012-cdef-123456789012',
          title: 'Refactor API validation middleware for performance',
          provider: rotateProvider(2),
          estimated_cost_usd: '28.90',
          duration_ms: 3600000,
          git_repo: 'org/backend-api',
        },
        {
          id: 'd4e5f6a7-b8c9-0123-defa-234567890123',
          title: 'Add session analytics dashboard with Recharts',
          provider: rotateProvider(3),
          estimated_cost_usd: '22.50',
          duration_ms: 4800000,
          git_repo: 'org/frontend-app',
        },
        {
          id: 'e5f6a7b8-c9d0-1234-efab-345678901234',
          title: 'Write integration tests for user management',
          provider: rotateProvider(4),
          estimated_cost_usd: '18.75',
          duration_ms: 2700000,
        },
        {
          id: 'f6a7b8c9-d0e1-2345-fabc-456789012345',
          title: 'Migrate database schema to support multi-tenancy',
          provider: rotateProvider(5),
          estimated_cost_usd: '15.30',
          duration_ms: 1800000,
          git_repo: 'org/backend-api',
        },
        {
          id: 'a7b8c9d0-e1f2-3456-abcd-567890123456',
          title: 'Set up CI/CD pipeline with GitHub Actions',
          provider: rotateProvider(6),
          estimated_cost_usd: '12.40',
          duration_ms: 3200000,
          git_repo: 'org/infra',
        },
        {
          id: 'b8c9d0e1-f2a3-4567-bcde-678901234567',
          title: 'Optimize webpack bundle size',
          provider: rotateProvider(7),
          estimated_cost_usd: '8.90',
          duration_ms: 2100000,
          git_repo: 'org/frontend-app',
        },
        {
          id: 'c9d0e1f2-a3b4-5678-cdef-789012345678',
          title: 'Fix race condition in WebSocket handler',
          provider: rotateProvider(8),
          estimated_cost_usd: '5.25',
          duration_ms: 900000,
          git_repo: 'org/realtime-service',
        },
        {
          // Exercises the unknown-provider fallback in `providerIcon` —
          // intentional empty string, not a defaulted provider literal.
          id: 'd0e1f2a3-b4c5-6789-defa-890123456789',
          title: 'Untitled session - a1b2c3d4',
          provider: '',
          estimated_cost_usd: '0.8500',
          duration_ms: 600000,
        },
      ],
    },
  },
};

// h7xe: with an onTopNChange handler the card renders the 10/25/50 selector.
export const WithTopNSelector: Story = {
  args: {
    ...Default.args,
    topN: 25,
    onTopNChange: () => {},
  },
};

// h7xe: while a selector-driven refetch is in flight the list dims and the
// segmented control is disabled.
export const Loading: Story = {
  args: {
    ...Default.args,
    topN: 10,
    onTopNChange: () => {},
    loading: true,
  },
};

export const FewSessions: Story = {
  args: {
    data: {
      sessions: [
        {
          id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
          title: 'Major refactoring of authentication system',
          provider: rotateProvider(0),
          estimated_cost_usd: '25.00',
          duration_ms: 5400000,
          git_repo: 'org/auth-service',
        },
        {
          id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
          title: 'Add comprehensive test coverage',
          provider: rotateProvider(1),
          estimated_cost_usd: '12.50',
          duration_ms: 3600000,
          git_repo: 'org/backend-api',
        },
        {
          id: 'c3d4e5f6-a7b8-9012-cdef-123456789012',
          title: 'Quick bug fix in CSS layout',
          provider: rotateProvider(2),
          estimated_cost_usd: '0.7500',
          git_repo: 'org/frontend-app',
        },
      ],
    },
  },
};

export const SingleSession: Story = {
  args: {
    data: {
      sessions: [
        {
          id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
          title: 'Full-stack feature implementation with tests',
          provider: rotateProvider(0),
          estimated_cost_usd: '42.00',
          duration_ms: 7200000,
          git_repo: 'org/my-project',
        },
      ],
    },
  },
};

export const NullData: Story = {
  args: {
    data: null,
  },
};
