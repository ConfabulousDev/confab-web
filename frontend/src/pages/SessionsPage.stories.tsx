import type { Meta, StoryObj } from '@storybook/react';
import Chip from '@/components/Chip';
import { RepoIcon, BranchIcon, GitHubIcon, DurationIcon, PRIcon, ClaudeCodeIcon } from '@/components/icons';
import { formatRelativeTime, formatDuration } from '@/utils';
import styles from './SessionsPage.module.css';

// Type for mock session data
interface MockSession {
  id: string;
  external_id: string;
  custom_title: string | null;
  summary: string | null;
  first_user_message: string | null;
  first_seen: string;
  last_sync_time: string | null;
  git_repo: string | null;
  git_repo_url: string | null;
  git_branch: string | null;
  github_prs?: string[] | null;
  shared_by_email?: string | null;
}

// Mock session data representing different scenarios
const mockSessions: MockSession[] = [
  {
    id: '1',
    external_id: '3b9cbb80-1234-5678-9abc-def012345678',
    custom_title: null,
    summary: 'Recently we started ingesting hostname and username in sync/init API. I want to start displaying this in the session list.',
    first_user_message: null,
    first_seen: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 18 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
    github_prs: ['142'],
  },
  {
    id: '2',
    external_id: 'b79fc2f8-2345-6789-abcd-ef0123456789',
    custom_title: null,
    summary: 'check the latest pending changes in the api md files. See if you understand what changed.',
    first_user_message: null,
    first_seen: new Date(Date.now() - 25 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
  },
  {
    id: '3',
    external_id: '82211e78-3456-789a-bcde-f01234567890',
    custom_title: null,
    summary: 'Backend API metadata nesting & client telemetry',
    first_user_message: null,
    first_seen: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(),
    git_repo: 'internal/confab',
    git_repo_url: 'https://gitlab.company.com/internal/confab',
    git_branch: 'main',
  },
  {
    id: '4',
    external_id: 'cd41c859-4567-89ab-cdef-012345678901',
    custom_title: null,
    summary: 'Sync API Metadata Nesting Implementation',
    first_user_message: null,
    first_seen: new Date(Date.now() - 26 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
  },
  {
    id: '5',
    external_id: '5a7e3441-5678-9abc-def0-123456789012',
    custom_title: null,
    summary: 'Refactor onboarding UI into reusable Quickstart',
    first_user_message: null,
    first_seen: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'feature/quickstart',
    github_prs: ['118', '119'],
  },
  {
    id: '6',
    external_id: '6b8f4552-6789-abcd-ef01-234567890123',
    custom_title: null,
    summary: 'Add authentication middleware',
    first_user_message: null,
    first_seen: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000 - 45 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab',
    git_repo_url: 'https://github.com/ConfabulousDev/confab',
    git_branch: 'develop',
  },
];

// Presentational component for the session list table
interface SessionListTableProps {
  sessions: MockSession[];
}

function SessionListTable({ sessions }: SessionListTableProps) {
  return (
    <div className={styles.card}>
      <div className={styles.sessionsTable}>
        <table>
          <thead>
            <tr>
              <th>Session</th>
              <th>Activity</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map((session) => (
              <tr key={session.id} className={`${styles.clickableRow} ${session.shared_by_email ? styles.sharedRow : ''}`}>
                <td className={styles.sessionCell}>
                  <div className={session.custom_title || session.summary || session.first_user_message ? styles.sessionTitle : `${styles.sessionTitle} ${styles.untitled}`}>
                    {session.custom_title || session.summary || session.first_user_message || 'Untitled'}
                  </div>
                  <div className={styles.chipRow}>
                    <Chip icon={ClaudeCodeIcon} variant="neutral">
                      {session.external_id.substring(0, 8)}
                    </Chip>
                    {session.git_repo && (
                      <Chip
                        icon={session.git_repo_url?.includes('github.com') ? GitHubIcon : RepoIcon}
                        variant="neutral"
                      >
                        {session.git_repo}
                      </Chip>
                    )}
                    {session.git_branch && (
                      <Chip icon={BranchIcon} variant="blue">
                        {session.git_branch}
                      </Chip>
                    )}
                    {session.github_prs?.map((pr) => (
                      <Chip key={pr} icon={PRIcon} variant="purple">
                        #{pr}
                      </Chip>
                    ))}
                  </div>
                  {session.shared_by_email && (
                    <div className={styles.sharedByLine}>
                      Shared by {session.shared_by_email}
                    </div>
                  )}
                </td>
                <td className={styles.timestamp}>
                  <span className={styles.activityContent}>
                    <span className={styles.activityTime}>
                      {session.last_sync_time ? formatRelativeTime(session.last_sync_time) : '-'}
                    </span>
                    {session.first_seen && session.last_sync_time && (
                      <span className={styles.activityDuration}>
                        {DurationIcon}
                        {formatDuration(new Date(session.last_sync_time).getTime() - new Date(session.first_seen).getTime())}
                      </span>
                    )}
                  </span>
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
        first_seen: new Date(Date.now() - 35 * 60 * 1000).toISOString(),
        last_sync_time: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
        git_repo: 'company/another-repo',
        git_repo_url: 'https://github.com/company/another-repo',
        git_branch: 'main',
      },
    ],
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

// Sessions shared with the current user (from other people)
const mockSharedSessions: MockSession[] = [
  {
    id: 'shared-1',
    external_id: 'shared-abc12345',
    custom_title: null,
    summary: 'API endpoint refactoring for v2',
    first_user_message: null,
    first_seen: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'feature/api-v2',
    shared_by_email: 'alice@example.com',
  },
  {
    id: 'shared-2',
    external_id: 'shared-def67890',
    custom_title: 'Database migration planning',
    summary: null,
    first_user_message: null,
    first_seen: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 12 * 60 * 60 * 1000).toISOString(),
    git_repo: 'company/backend',
    git_repo_url: 'https://github.com/company/backend',
    git_branch: 'main',
    shared_by_email: 'bob@company.com',
  },
  {
    id: 'shared-3',
    external_id: 'shared-ghi11223',
    custom_title: null,
    summary: 'Frontend performance optimization',
    first_user_message: null,
    first_seen: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'perf/lazy-loading',
    github_prs: ['156'],
    shared_by_email: 'carol@example.org',
  },
];

export const WithSharedSessions: Story = {
  args: {
    sessions: [...mockSessions.slice(0, 2), ...mockSharedSessions],
  },
};
