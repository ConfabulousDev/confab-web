import type { Meta, StoryObj } from '@storybook/react-vite';
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

export const Default: Story = {
  args: {
    data: {
      sessions: [
        {
          id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
          title: 'Implement comprehensive dark mode with theme system',
          estimated_cost_usd: '45.20',
          duration_ms: 7200000,
          git_repo: 'org/frontend-app',
        },
        {
          id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
          title: 'Debug OAuth login redirect loop in production',
          estimated_cost_usd: '32.15',
          duration_ms: 5400000,
          git_repo: 'org/auth-service',
        },
        {
          id: 'c3d4e5f6-a7b8-9012-cdef-123456789012',
          title: 'Refactor API validation middleware for performance',
          estimated_cost_usd: '28.90',
          duration_ms: 3600000,
          git_repo: 'org/backend-api',
        },
        {
          id: 'd4e5f6a7-b8c9-0123-defa-234567890123',
          title: 'Add session analytics dashboard with Recharts',
          estimated_cost_usd: '22.50',
          duration_ms: 4800000,
          git_repo: 'org/frontend-app',
        },
        {
          id: 'e5f6a7b8-c9d0-1234-efab-345678901234',
          title: 'Write integration tests for user management',
          estimated_cost_usd: '18.75',
          duration_ms: 2700000,
        },
        {
          id: 'f6a7b8c9-d0e1-2345-fabc-456789012345',
          title: 'Migrate database schema to support multi-tenancy',
          estimated_cost_usd: '15.30',
          duration_ms: 1800000,
          git_repo: 'org/backend-api',
        },
        {
          id: 'a7b8c9d0-e1f2-3456-abcd-567890123456',
          title: 'Set up CI/CD pipeline with GitHub Actions',
          estimated_cost_usd: '12.40',
          duration_ms: 3200000,
          git_repo: 'org/infra',
        },
        {
          id: 'b8c9d0e1-f2a3-4567-bcde-678901234567',
          title: 'Optimize webpack bundle size',
          estimated_cost_usd: '8.90',
          duration_ms: 2100000,
          git_repo: 'org/frontend-app',
        },
        {
          id: 'c9d0e1f2-a3b4-5678-cdef-789012345678',
          title: 'Fix race condition in WebSocket handler',
          estimated_cost_usd: '5.25',
          duration_ms: 900000,
          git_repo: 'org/realtime-service',
        },
        {
          id: 'd0e1f2a3-b4c5-6789-defa-890123456789',
          title: 'Untitled session - a1b2c3d4',
          estimated_cost_usd: '0.8500',
          duration_ms: 600000,
        },
      ],
    },
  },
};

export const FewSessions: Story = {
  args: {
    data: {
      sessions: [
        {
          id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
          title: 'Major refactoring of authentication system',
          estimated_cost_usd: '25.00',
          duration_ms: 5400000,
          git_repo: 'org/auth-service',
        },
        {
          id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
          title: 'Add comprehensive test coverage',
          estimated_cost_usd: '12.50',
          duration_ms: 3600000,
          git_repo: 'org/backend-api',
        },
        {
          id: 'c3d4e5f6-a7b8-9012-cdef-123456789012',
          title: 'Quick bug fix in CSS layout',
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
