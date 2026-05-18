import type { Meta, StoryObj } from '@storybook/react-vite';
import OrgTable from './OrgTable';
import { makeOrgUserFixture } from '@/test-fixtures/org';

const meta: Meta<typeof OrgTable> = {
  title: 'Org/OrgTable',
  component: OrgTable,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <div style={{ padding: '16px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof OrgTable>;

const alice = makeOrgUserFixture('claude-code', {
  user: { id: 1, email: 'alice@example.com', name: 'Alice Chen' },
});

const bob = makeOrgUserFixture('claude-code', {
  user: { id: 2, email: 'bob@example.com', name: 'Bob Smith' },
  session_count: 23,
  total_cost_usd: '67.30',
  total_duration_ms: 198000000,
  total_assistant_time_ms: 108000000,
  total_user_time_ms: 90000000,
  avg_cost_usd: '2.93',
  avg_duration_ms: 8608695,
  avg_assistant_time_ms: 4695652,
  avg_user_time_ms: 3913043,
});

const carol = makeOrgUserFixture('claude-code', {
  user: { id: 3, email: 'carol@example.com', name: 'Carol Davis' },
  session_count: 12,
  total_cost_usd: '34.20',
  total_duration_ms: 86400000,
  total_assistant_time_ms: 50400000,
  total_user_time_ms: 36000000,
  avg_cost_usd: '2.85',
  avg_duration_ms: 7200000,
  avg_assistant_time_ms: 4200000,
  avg_user_time_ms: 3000000,
});

const dee = makeOrgUserFixture('codex', {
  user: { id: 4, email: 'dee@example.com', name: 'Dee Codex' },
});

const noNameUser = makeOrgUserFixture('claude-code', {
  user: { id: 5, email: 'dev@example.com', name: null },
  session_count: 8,
  total_cost_usd: '18.90',
  total_duration_ms: 57600000,
  total_assistant_time_ms: 28800000,
  total_user_time_ms: 28800000,
  avg_cost_usd: '2.36',
  avg_duration_ms: 7200000,
  avg_assistant_time_ms: 3600000,
  avg_user_time_ms: 3600000,
});

const zeroUser = makeOrgUserFixture('claude-code', {
  user: { id: 6, email: 'new@example.com', name: 'New User' },
  session_count: 0,
  total_cost_usd: '0',
  total_duration_ms: 0,
  total_assistant_time_ms: 0,
  total_user_time_ms: 0,
  avg_cost_usd: '0',
  avg_duration_ms: null,
  avg_assistant_time_ms: null,
  avg_user_time_ms: null,
});

export const Default: Story = {
  args: {
    users: [alice, bob, carol, noNameUser],
  },
};

export const SingleUser: Story = {
  args: {
    users: [alice],
  },
};

export const WithZeroSessionUsers: Story = {
  args: {
    users: [alice, bob, zeroUser],
  },
};

export const MixedProviders: Story = {
  args: {
    // Some users on Claude, some on Codex — exercises the relabeled
    // "Assistant Time" columns in a multi-provider context.
    users: [alice, bob, dee],
  },
};

export const CodexOnly: Story = {
  args: {
    users: [
      makeOrgUserFixture('codex', {
        user: { id: 10, email: 'erin@example.com', name: 'Erin' },
      }),
      makeOrgUserFixture('codex', {
        user: { id: 11, email: 'frank@example.com', name: 'Frank' },
        session_count: 8,
        total_cost_usd: '24.10',
        total_duration_ms: 72_000_000,
        total_assistant_time_ms: 48_000_000,
        total_user_time_ms: 24_000_000,
        avg_cost_usd: '3.01',
        avg_duration_ms: 9_000_000,
        avg_assistant_time_ms: 6_000_000,
        avg_user_time_ms: 3_000_000,
      }),
    ],
  },
};

export const HighUsage: Story = {
  args: {
    users: [
      makeOrgUserFixture('claude-code', {
        user: { id: 100, email: 'power@example.com', name: 'Power User' },
        session_count: 500,
        total_cost_usd: '2450.75',
        total_duration_ms: 8640000000,
        total_assistant_time_ms: 4320000000,
        total_user_time_ms: 4320000000,
        avg_cost_usd: '4.90',
        avg_duration_ms: 17280000,
        avg_assistant_time_ms: 8640000,
        avg_user_time_ms: 8640000,
      }),
      alice,
      bob,
    ],
  },
};
