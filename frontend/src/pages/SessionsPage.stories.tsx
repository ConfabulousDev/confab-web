import type { Meta, StoryObj } from '@storybook/react';
import Chip from '@/components/Chip';
import { RepoIcon, BranchIcon, ComputerIcon, GitHubIcon } from '@/components/icons';
import SortableHeader from '@/components/SortableHeader';
import styles from './SessionsPage.module.css';

// Type for mock session data
interface MockSession {
  id: string;
  external_id: string;
  custom_title: string | null;
  summary: string | null;
  first_user_message: string | null;
  last_sync_time: string | null;
  git_repo: string | null;
  git_repo_url: string | null;
  git_branch: string | null;
  hostname: string | null;
  username: string | null;
}

// Mock session data representing different scenarios
const mockSessions: MockSession[] = [
  {
    id: '1',
    external_id: '3b9cbb80-1234-5678-9abc-def012345678',
    custom_title: null,
    summary: 'Recently we started ingesting hostname and username in sync/init API. I want to start displaying this in the session list.',
    first_user_message: null,
    last_sync_time: new Date(Date.now() - 18 * 1000).toISOString(), // 18s ago
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
    hostname: 'macbook-pro.local',
    username: 'sarah',
  },
  {
    id: '2',
    external_id: 'b79fc2f8-2345-6789-abcd-ef0123456789',
    custom_title: null,
    summary: 'check the latest pending changes in the api md files. See if you understand what changed.',
    first_user_message: null,
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(), // 23h ago
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
    hostname: 'dev-server-01',
    username: 'sarah',
  },
  {
    id: '3',
    external_id: '82211e78-3456-789a-bcde-f01234567890',
    custom_title: null,
    summary: 'Backend API metadata nesting & client telemetry',
    first_user_message: null,
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(), // 23h ago
    git_repo: 'internal/confab',
    git_repo_url: 'https://gitlab.company.com/internal/confab', // Non-GitHub example
    git_branch: 'main',
    hostname: 'workstation',
    username: 'mike',
  },
  {
    id: '4',
    external_id: 'cd41c859-4567-89ab-cdef-012345678901',
    custom_title: null,
    summary: 'Sync API Metadata Nesting Implementation',
    first_user_message: null,
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(), // 23h ago
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
    hostname: 'macbook-pro.local',
    username: 'sarah',
  },
  {
    id: '5',
    external_id: '5a7e3441-5678-9abc-def0-123456789012',
    custom_title: null,
    summary: 'Refactor onboarding UI into reusable Quickstart',
    first_user_message: null,
    last_sync_time: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(), // 1d ago
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'feature/quickstart',
    hostname: 'ubuntu-desktop',
    username: 'alex',
  },
  {
    id: '6',
    external_id: '6b8f4552-6789-abcd-ef01-234567890123',
    custom_title: null,
    summary: 'Add authentication middleware',
    first_user_message: null,
    last_sync_time: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(), // 2d ago
    git_repo: 'ConfabulousDev/confab-cli',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-cli',
    git_branch: 'develop',
    hostname: null,
    username: null,
  },
];

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffSecs < 60) return `${diffSecs}s`;
  if (diffMins < 60) return `${diffMins}m`;
  if (diffHours < 24) return `${diffHours}h`;
  return `${diffDays}d`;
}

// Presentational component for the session list table
interface SessionListTableProps {
  sessions: MockSession[];
  sortColumn?: string;
  sortDirection?: 'asc' | 'desc';
}

function SessionListTable({ sessions, sortColumn = 'last_sync_time', sortDirection = 'desc' }: SessionListTableProps) {
  return (
    <div className={styles.card}>
      <div className={styles.sessionsTable}>
        <table>
          <thead>
            <tr>
              <SortableHeader
                column="summary"
                label="Title"
                currentColumn={sortColumn}
                direction={sortDirection}
                onSort={() => {}}
              />
              <th className={styles.shrinkCol}>Git</th>
              <th className={styles.shrinkCol}>Hostname</th>
              <SortableHeader
                column="external_id"
                label="CC id"
                currentColumn={sortColumn}
                direction={sortDirection}
                onSort={() => {}}
              />
              <SortableHeader
                column="last_sync_time"
                label="Activity"
                currentColumn={sortColumn}
                direction={sortDirection}
                onSort={() => {}}
              />
            </tr>
          </thead>
          <tbody>
            {sessions.map((session) => (
              <tr key={session.id} className={styles.clickableRow}>
                <td className={styles.titleCell}>
                  <span>{session.custom_title || session.summary || session.first_user_message || 'Untitled'}</span>
                </td>
                <td className={styles.shrinkCol}>
                  <div className={styles.chipCell}>
                    {session.git_repo && (
                      <Chip
                        icon={session.git_repo_url?.includes('github.com') ? GitHubIcon : RepoIcon}
                        variant="neutral"
                        title={session.git_repo}
                        ellipsis="start"
                      >
                        {session.git_repo}
                      </Chip>
                    )}
                    {session.git_branch && (
                      <Chip icon={BranchIcon} variant="blue" title={session.git_branch}>
                        {session.git_branch}
                      </Chip>
                    )}
                  </div>
                </td>
                <td className={styles.shrinkCol}>
                  {session.hostname && (
                    <Chip icon={ComputerIcon} variant="green" title={session.hostname}>
                      {session.hostname}
                    </Chip>
                  )}
                </td>
                <td className={styles.sessionId}>
                  {session.external_id.substring(0, 8)}
                </td>
                <td className={styles.timestamp}>
                  {session.last_sync_time ? formatRelativeTime(session.last_sync_time) : '-'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

const meta: Meta<typeof SessionListTable> = {
  title: 'Pages/SessionsPage',
  component: SessionListTable,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof SessionListTable>;

export const Default: Story = {
  args: {
    sessions: mockSessions,
  },
};

export const WithMixedData: Story = {
  args: {
    sessions: [
      ...mockSessions,
      {
        id: '7',
        external_id: '7c9g5663-789a-bcde-f012-345678901234',
        custom_title: 'Custom titled session',
        summary: 'This has a custom title set by the user',
        first_user_message: null,
        last_sync_time: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
        git_repo: 'company/another-repo',
        git_repo_url: 'https://github.com/company/another-repo',
        git_branch: 'main',
        hostname: 'server.internal.company.com',
        username: 'deploy',
      },
    ],
  },
};

export const NoSystemInfo: Story = {
  args: {
    sessions: mockSessions.map(s => ({ ...s, hostname: null, username: null })),
  },
};

export const NoGitInfo: Story = {
  args: {
    sessions: mockSessions.map(s => ({ ...s, git_repo: null, git_branch: null })),
  },
};

export const Empty: Story = {
  args: {
    sessions: [],
  },
  render: () => (
    <div className={styles.card}>
      <p className={styles.empty}>No sessions found</p>
    </div>
  ),
};
