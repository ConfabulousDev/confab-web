import type { Meta, StoryObj } from '@storybook/react-vite';
import PageHeader from '@/components/PageHeader';
import TrendsFilters, { type TrendsFiltersValue } from '@/components/trends/TrendsFilters';
import {
  TrendsOverviewCard,
  TrendsTokensCard,
  TrendsActivityCard,
  TrendsToolsCard,
  TrendsUtilizationCard,
} from '@/components/trends/cards';
import Alert from '@/components/Alert';
import type { TrendsResponse } from '@/schemas/api';
import styles from './TrendsPage.module.css';

// Presentational component for Storybook
interface TrendsPagePresentationalProps {
  data: TrendsResponse | null;
  loading?: boolean;
  error?: Error | null;
  repos?: string[];
  filters?: TrendsFiltersValue;
}

function TrendsPagePresentational({
  data,
  loading = false,
  error = null,
  repos = [],
  filters = {
    dateRange: { startDate: '2024-01-08', endDate: '2024-01-14', label: 'Last 7 Days' },
    repos: [],
    includeNoRepo: true,
  },
}: TrendsPagePresentationalProps) {
  const showEmptyState = !loading && data && data.session_count === 0;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Trends</h1>}
          actions={
            <TrendsFilters
              repos={repos}
              value={filters}
              onChange={() => {}}
            />
          }
        />

        <div className={styles.container}>
          {error && <Alert variant="error">{error.message}</Alert>}

          {loading && !data && (
            <div className={styles.loading}>Loading trends...</div>
          )}

          {showEmptyState && (
            <div className={styles.emptyState}>
              <div className={styles.emptyStateTitle}>No sessions found</div>
              <div className={styles.emptyStateText}>
                No sessions match the selected filters. Try adjusting the date range or repo filter.
              </div>
            </div>
          )}

          {data && data.session_count > 0 && (
            <div className={styles.cardsGrid}>
              <TrendsOverviewCard data={data.cards.overview} />
              <TrendsTokensCard data={data.cards.tokens} />
              <TrendsActivityCard data={data.cards.activity} />
              <TrendsToolsCard data={data.cards.tools} />
              <TrendsUtilizationCard data={data.cards.utilization} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const meta: Meta<typeof TrendsPagePresentational> = {
  title: 'Pages/TrendsPage',
  component: TrendsPagePresentational,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof TrendsPagePresentational>;

// Mock data generators
const mockDailyCosts = [
  { date: '2024-01-08', cost_usd: '1.25' },
  { date: '2024-01-09', cost_usd: '1.80' },
  { date: '2024-01-10', cost_usd: '0.95' },
  { date: '2024-01-11', cost_usd: '2.20' },
  { date: '2024-01-12', cost_usd: '1.50' },
  { date: '2024-01-13', cost_usd: '0.65' },
  { date: '2024-01-14', cost_usd: '1.90' },
];

const mockDailySessionCounts = [
  { date: '2024-01-08', session_count: 5 },
  { date: '2024-01-09', session_count: 8 },
  { date: '2024-01-10', session_count: 3 },
  { date: '2024-01-11', session_count: 12 },
  { date: '2024-01-12', session_count: 6 },
  { date: '2024-01-13', session_count: 2 },
  { date: '2024-01-14', session_count: 10 },
];

const mockDailyUtilization = [
  { date: '2024-01-08', utilization_pct: 45.2 },
  { date: '2024-01-09', utilization_pct: 52.8 },
  { date: '2024-01-10', utilization_pct: 38.5 },
  { date: '2024-01-11', utilization_pct: 61.3 },
  { date: '2024-01-12', utilization_pct: 55.0 },
  { date: '2024-01-13', utilization_pct: 42.1 },
  { date: '2024-01-14', utilization_pct: 48.7 },
];

const defaultMockData: TrendsResponse = {
  computed_at: '2024-01-15T10:30:00Z',
  date_range: { start_date: '2024-01-08', end_date: '2024-01-14' },
  session_count: 42,
  repos_included: ['org/repo-web', 'org/repo-api'],
  include_no_repo: true,
  cards: {
    overview: {
      session_count: 42,
      total_duration_ms: 86400000,
      avg_duration_ms: 2057142,
      days_covered: 7,
      total_assistant_duration_ms: 43200000,
      assistant_utilization_pct: 50.0,
    },
    tokens: {
      total_input_tokens: 1250000,
      total_output_tokens: 450000,
      total_cache_read_tokens: 350000,
      total_cache_creation_tokens: 50000,
      total_cost_usd: '10.25',
      daily_costs: mockDailyCosts,
    },
    activity: {
      total_files_read: 500,
      total_files_modified: 150,
      total_lines_added: 5000,
      total_lines_removed: 2000,
      daily_session_counts: mockDailySessionCounts,
    },
    tools: {
      total_calls: 2500,
      total_errors: 50,
      tool_stats: {
        Read: { success: 800, errors: 5 },
        Write: { success: 400, errors: 10 },
        Edit: { success: 350, errors: 8 },
        Bash: { success: 600, errors: 30 },
        Grep: { success: 200, errors: 2 },
        Glob: { success: 150, errors: 0 },
      },
    },
    utilization: {
      daily_utilization: mockDailyUtilization,
    },
  },
};

export const Default: Story = {
  args: {
    data: defaultMockData,
    repos: ['org/repo-web', 'org/repo-api', 'org/repo-cli'],
  },
};

export const HighUsage: Story = {
  args: {
    data: {
      computed_at: '2024-01-15T10:30:00Z',
      date_range: { start_date: '2023-12-15', end_date: '2024-01-14' },
      session_count: 250,
      repos_included: ['org/repo-web', 'org/repo-api', 'org/repo-cli'],
      include_no_repo: true,
      cards: {
        overview: {
          session_count: 250,
          total_duration_ms: 604800000,
          avg_duration_ms: 2419200,
          days_covered: 30,
          total_assistant_duration_ms: 302400000,
          assistant_utilization_pct: 50.0,
        },
        tokens: {
          total_input_tokens: 15000000,
          total_output_tokens: 5500000,
          total_cache_read_tokens: 4200000,
          total_cache_creation_tokens: 800000,
          total_cost_usd: '125.00',
          daily_costs: [
            { date: '2024-01-08', cost_usd: '4.50' },
            { date: '2024-01-09', cost_usd: '5.20' },
            { date: '2024-01-10', cost_usd: '3.80' },
            { date: '2024-01-11', cost_usd: '6.20' },
            { date: '2024-01-12', cost_usd: '4.90' },
            { date: '2024-01-13', cost_usd: '2.80' },
            { date: '2024-01-14', cost_usd: '5.80' },
          ],
        },
        activity: {
          total_files_read: 15000,
          total_files_modified: 3500,
          total_lines_added: 125000,
          total_lines_removed: 45000,
          daily_session_counts: [
            { date: '2024-01-08', session_count: 15 },
            { date: '2024-01-09', session_count: 20 },
            { date: '2024-01-10', session_count: 18 },
            { date: '2024-01-11', session_count: 25 },
            { date: '2024-01-12', session_count: 12 },
            { date: '2024-01-13', session_count: 8 },
            { date: '2024-01-14', session_count: 22 },
          ],
        },
        tools: {
          total_calls: 12000,
          total_errors: 250,
          tool_stats: {
            Read: { success: 3500, errors: 20 },
            Write: { success: 1200, errors: 45 },
            Edit: { success: 1800, errors: 35 },
            Bash: { success: 2500, errors: 120 },
            Grep: { success: 1500, errors: 10 },
            Glob: { success: 800, errors: 5 },
            Task: { success: 450, errors: 10 },
            WebFetch: { success: 200, errors: 5 },
          },
        },
        utilization: {
          daily_utilization: [
            { date: '2024-01-08', utilization_pct: 55.2 },
            { date: '2024-01-09', utilization_pct: 62.8 },
            { date: '2024-01-10', utilization_pct: 48.5 },
            { date: '2024-01-11', utilization_pct: 71.3 },
            { date: '2024-01-12', utilization_pct: 65.0 },
            { date: '2024-01-13', utilization_pct: 52.1 },
            { date: '2024-01-14', utilization_pct: 58.7 },
          ],
        },
      },
    },
    repos: ['org/repo-web', 'org/repo-api', 'org/repo-cli'],
    filters: {
      dateRange: { startDate: '2023-12-15', endDate: '2024-01-14', label: 'Last 30 Days' },
      repos: [],
      includeNoRepo: true,
    },
  },
};

export const SingleSession: Story = {
  args: {
    data: {
      computed_at: '2024-01-15T10:30:00Z',
      date_range: { start_date: '2024-01-14', end_date: '2024-01-14' },
      session_count: 1,
      repos_included: ['org/repo-web'],
      include_no_repo: false,
      cards: {
        overview: {
          session_count: 1,
          total_duration_ms: 3600000,
          avg_duration_ms: 3600000,
          days_covered: 1,
          total_assistant_duration_ms: 2700000,
          assistant_utilization_pct: 75.0,
        },
        tokens: {
          total_input_tokens: 25000,
          total_output_tokens: 8000,
          total_cache_read_tokens: 5000,
          total_cache_creation_tokens: 1000,
          total_cost_usd: '0.35',
          daily_costs: [{ date: '2024-01-14', cost_usd: '0.35' }],
        },
        activity: {
          total_files_read: 50,
          total_files_modified: 10,
          total_lines_added: 200,
          total_lines_removed: 50,
          daily_session_counts: [{ date: '2024-01-14', session_count: 1 }],
        },
        tools: {
          total_calls: 80,
          total_errors: 2,
          tool_stats: {
            Read: { success: 40, errors: 0 },
            Edit: { success: 25, errors: 2 },
            Bash: { success: 15, errors: 0 },
          },
        },
        utilization: {
          daily_utilization: [{ date: '2024-01-14', utilization_pct: 75.0 }],
        },
      },
    },
    repos: ['org/repo-web'],
    filters: {
      dateRange: { startDate: '2024-01-14', endDate: '2024-01-14', label: 'This Week' },
      repos: ['org/repo-web'],
      includeNoRepo: false,
    },
  },
};

export const WithMCPTools: Story = {
  args: {
    data: {
      ...defaultMockData,
      cards: {
        ...defaultMockData.cards,
        tools: {
          total_calls: 500,
          total_errors: 15,
          tool_stats: {
            Read: { success: 150, errors: 0 },
            Write: { success: 80, errors: 5 },
            Edit: { success: 70, errors: 3 },
            'mcp__linear-server__list_issues': { success: 45, errors: 2 },
            'mcp__linear-server__create_issue': { success: 30, errors: 3 },
            'mcp__github__create_pr': { success: 25, errors: 2 },
            'mcp__github__list_prs': { success: 20, errors: 0 },
          },
        },
      },
    },
    repos: ['org/repo-web'],
  },
};

export const EmptyState: Story = {
  args: {
    data: {
      computed_at: '2024-01-15T10:30:00Z',
      date_range: { start_date: '2024-01-08', end_date: '2024-01-14' },
      session_count: 0,
      repos_included: [],
      include_no_repo: true,
      cards: {
        overview: null,
        tokens: null,
        activity: null,
        tools: null,
        utilization: null,
      },
    },
    repos: [],
  },
};

export const Loading: Story = {
  args: {
    data: null,
    loading: true,
    repos: ['org/repo-web', 'org/repo-api'],
  },
};

export const ErrorState: Story = {
  args: {
    data: null,
    error: new globalThis.Error('Failed to fetch trends data. Please try again.'),
    repos: ['org/repo-web'],
  },
};

export const FilteredByRepo: Story = {
  args: {
    data: {
      ...defaultMockData,
      session_count: 15,
      repos_included: ['org/repo-api'],
      include_no_repo: false,
      cards: {
        ...defaultMockData.cards,
        overview: {
          session_count: 15,
          total_duration_ms: 28800000,
          avg_duration_ms: 1920000,
          days_covered: 7,
          total_assistant_duration_ms: 14400000,
          assistant_utilization_pct: 50.0,
        },
      },
    },
    repos: ['org/repo-web', 'org/repo-api', 'org/repo-cli'],
    filters: {
      dateRange: { startDate: '2024-01-08', endDate: '2024-01-14', label: 'Last 7 Days' },
      repos: ['org/repo-api'],
      includeNoRepo: false,
    },
  },
};
