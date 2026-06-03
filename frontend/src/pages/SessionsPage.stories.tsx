import type { Meta, StoryObj } from '@storybook/react';
import Chip from '@/components/Chip';
import PageHeader from '@/components/PageHeader';
import FilterChipsBar from '@/components/FilterChipsBar';
import Pagination from '@/components/Pagination';
import { RepoIcon, BranchIcon, GitHubIcon, DurationIcon, PRIcon, RefreshIcon } from '@/components/icons';
import { getProviderIcon } from '@/components/providerIcon';
import { formatRelativeTime, formatDuration } from '@/utils';
import { formatCost } from '@/utils/tokenStats';
import type { SessionFilterOptions } from '@/schemas/api';
import styles from './SessionsPage.module.css';

const mockFilterOptions: SessionFilterOptions = {
  repos: ['ConfabulousDev/confab-web', 'ConfabulousDev/confab', 'internal/confab'],
  branches: ['main', 'develop', 'feature/quickstart', 'feature/codex-attachments'],
  owners: ['alice@example.com', 'bob@example.com', 'carol@example.com'],
  providers: ['claude-code', 'codex'],
};

const noopFilterHandlers = {
  onToggleRepo: () => {},
  onToggleBranch: () => {},
  onToggleOwner: () => {},
  onToggleProvider: () => {},
  onQueryChange: () => {},
  onClearAll: () => {},
};

// Type for mock session data
interface MockSession {
  id: string;
  external_id: string;
  // Canonical agent identifier; drives the chip icon (orange Claude or
  // teal Codex). Defaults to 'claude-code' when omitted on a mock row.
  provider?: string;
  custom_title: string | null;
  // Title proposed by the smart-recap pipeline (CF-350 / CF-447). Falls
  // between custom_title and summary in the SessionsPage title chain.
  suggested_session_title?: string | null;
  summary: string | null;
  first_user_message: string | null;
  first_seen: string;
  last_sync_time: string | null;
  estimated_cost_usd?: string | null;
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
    estimated_cost_usd: '4.2300',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
    github_prs: ['https://github.com/ConfabulousDev/confab-web/pull/142'],
  },
  {
    id: '2',
    external_id: 'b79fc2f8-2345-6789-abcd-ef0123456789',
    custom_title: null,
    summary: 'check the latest pending changes in the api md files. See if you understand what changed.',
    first_user_message: null,
    first_seen: new Date(Date.now() - 25 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 23 * 60 * 60 * 1000).toISOString(),
    estimated_cost_usd: '0.1200',
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
    estimated_cost_usd: null,
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
    estimated_cost_usd: '12.8700',
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
    estimated_cost_usd: '0.0500',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'feature/quickstart',
    github_prs: ['https://github.com/ConfabulousDev/confab-web/pull/118', 'https://github.com/ConfabulousDev/confab-web/pull/119'],
  },
  {
    id: '6',
    external_id: '6b8f4552-6789-abcd-ef01-234567890123',
    custom_title: null,
    summary: 'Add authentication middleware',
    first_user_message: null,
    first_seen: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000 - 45 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
    estimated_cost_usd: '1.5600',
    git_repo: 'ConfabulousDev/confab',
    git_repo_url: 'https://github.com/ConfabulousDev/confab',
    git_branch: 'develop',
  },
  {
    id: '7',
    external_id: '019e23cc-7890-bcde-f012-345678901234',
    provider: 'codex',
    custom_title: null,
    summary: 'Investigate Codex rollout schema for transcript parser',
    first_user_message: null,
    first_seen: new Date(Date.now() - 90 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
    estimated_cost_usd: '0.4200',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
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
              <th>Title</th>
              <th className={styles.costHeader}>Est. Cost</th>
              <th>Activity</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map((session) => {
              const title =
                session.custom_title ||
                session.suggested_session_title ||
                session.summary ||
                session.first_user_message;
              return (
                <tr key={session.id} className={`${styles.clickableRow} ${session.shared_by_email ? styles.sharedRow : ''}`}>
                  <td className={styles.sessionCell}>
                    <div className={title ? styles.sessionTitle : `${styles.sessionTitle} ${styles.untitled}`}>
                      {title || 'Untitled'}
                    </div>
                    <div className={styles.chipRow}>
                      <Chip icon={getProviderIcon(session.provider ?? 'claude-code')} variant="neutral">
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
                      {session.github_prs?.map((prUrl) => (
                        <Chip key={prUrl} icon={PRIcon} variant="purple">
                          #{prUrl.split('/').pop() ?? prUrl}
                        </Chip>
                      ))}
                    </div>
                    {session.shared_by_email && (
                      <div className={styles.sharedByLine}>
                        Shared by {session.shared_by_email}
                      </div>
                    )}
                  </td>
                  <td className={styles.costCell}>
                    {session.estimated_cost_usd
                      ? formatCost(parseFloat(session.estimated_cost_usd))
                      : '-'}
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
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// Full page chrome — PageHeader (with refresh + Pagination) + filter bar +
// scrollable container — mirrors the live SessionsPage layout for verifying
// the shared toolbar / filter spacing end-to-end.
interface SessionsPageChromeProps {
  sessions: MockSession[];
  filters?: {
    repos: string[];
    branches: string[];
    owners: string[];
    providers: string[];
    query: string;
  };
}

const emptyFilters = { repos: [], branches: [], owners: [], providers: [], query: '' };

function SessionsPageChrome({ sessions, filters = emptyFilters }: SessionsPageChromeProps) {
  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Sessions</h1>}
          actions={
            <div className={styles.toolbarActions}>
              <Pagination hasMore canGoPrev onNext={() => {}} onPrev={() => {}} />
              <button
                className={styles.refreshBtn}
                aria-label="Refresh sessions"
                title="Refresh sessions"
              >
                {RefreshIcon}
              </button>
            </div>
          }
        />
        <div className={styles.filterBar}>
          <FilterChipsBar
            filters={filters}
            filterOptions={mockFilterOptions}
            currentUserEmail="alice@example.com"
            {...noopFilterHandlers}
          />
        </div>
        <div className={styles.container}>
          <SessionListTable sessions={sessions} />
        </div>
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
        estimated_cost_usd: '0.0030',
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
    estimated_cost_usd: '7.4500',
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
    estimated_cost_usd: null,
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
    estimated_cost_usd: '0.8900',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'perf/lazy-loading',
    github_prs: ['https://github.com/ConfabulousDev/confab-web/pull/156'],
    shared_by_email: 'carol@example.org',
  },
];

export const WithSharedSessions: Story = {
  args: {
    sessions: [...mockSessions.slice(0, 2), ...mockSharedSessions],
  },
};

// Codex-only sessions: confirms getProviderIcon renders the teal OpenAI
// blossom (CodexIcon) on every row instead of the orange Claude logo.
// Regression guard for CF-353 — pair with the providerIcon.test.tsx unit test.
// The third row also exercises the smart-recap → list-title surface
// (CF-350 + CF-447): when `suggested_session_title` is set, it wins over
// `summary` and `first_user_message` in the title fallback chain.
const mockCodexSessions: MockSession[] = [
  {
    id: 'codex-1',
    external_id: '019e23cc-1111-2222-3333-444455556666',
    provider: 'codex',
    custom_title: null,
    summary: 'Investigate Codex rollout schema for transcript parser',
    first_user_message: null,
    first_seen: new Date(Date.now() - 90 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
    estimated_cost_usd: '0.4200',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'main',
  },
  {
    id: 'codex-2',
    external_id: '019e23cc-aaaa-bbbb-cccc-ddddeeeeffff',
    provider: 'codex',
    custom_title: null,
    summary: 'Refactor Codex transcript pane to handle away_summary attachments',
    first_user_message: null,
    first_seen: new Date(Date.now() - 4 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 35 * 60 * 1000).toISOString(),
    estimated_cost_usd: '1.2300',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'feature/codex-attachments',
  },
  {
    id: 'codex-3',
    external_id: '019e23cc-7777-8888-9999-aaaabbbbcccc',
    provider: 'codex',
    custom_title: null,
    suggested_session_title: 'Wire Codex token-usage parser to analytics',
    summary: 'so basically i was looking at this stack trace and wondered',
    first_user_message: 'so basically i was looking at this stack trace and wondered',
    first_seen: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
    last_sync_time: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
    estimated_cost_usd: '2.8700',
    git_repo: 'ConfabulousDev/confab-web',
    git_repo_url: 'https://github.com/ConfabulousDev/confab-web',
    git_branch: 'feature/codex-tokens',
  },
];

export const CodexOnly: Story = {
  args: {
    sessions: mockCodexSessions,
  },
};

// Full-chrome stories — render PageHeader + FilterChipsBar + table together
// for end-to-end visual verification of toolbar/filter spacing & control heights.
export const FullPage: Story = {
  parameters: { layout: 'fullscreen' },
  render: () => <SessionsPageChrome sessions={mockSessions} />,
};

export const FullPageWithActiveFilters: Story = {
  parameters: { layout: 'fullscreen' },
  render: () => (
    <SessionsPageChrome
      sessions={mockSessions}
      filters={{
        repos: ['ConfabulousDev/confab-web'],
        branches: ['main'],
        owners: ['alice@example.com'],
        providers: ['claude-code'],
        query: 'auth',
      }}
    />
  ),
};
