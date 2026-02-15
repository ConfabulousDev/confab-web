import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useDocumentTitle, useTrends } from '@/hooks';
import { sessionsAPI } from '@/services/api';
import PageHeader from '@/components/PageHeader';
import TrendsFilters, { type TrendsFiltersValue, type DateRange } from '@/components/trends/TrendsFilters';
import {
  TrendsOverviewCard,
  TrendsTokensCard,
  TrendsActivityCard,
  TrendsToolsCard,
  TrendsUtilizationCard,
  TrendsAgentsAndSkillsCard,
} from '@/components/trends/cards';
import Alert from '@/components/Alert';
import styles from './TrendsPage.module.css';

// Format date as YYYY-MM-DD
function formatDate(date: Date): string {
  const iso = date.toISOString();
  return iso.slice(0, 10);
}

// Get default date range (last 7 days)
function getDefaultDateRange(): DateRange {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const last7Days = new Date(today);
  last7Days.setDate(last7Days.getDate() - 6);
  return {
    startDate: formatDate(last7Days),
    endDate: formatDate(today),
    label: 'Last 7 Days',
  };
}

// Get label for a date range (used when loading from URL)
function getDateRangeLabel(startDate: string, endDate: string): string {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayStr = formatDate(today);

  // Check common presets
  const daysDiff = Math.round(
    (new Date(endDate).getTime() - new Date(startDate).getTime()) / (1000 * 60 * 60 * 24)
  );

  if (endDate === todayStr) {
    if (daysDiff === 6) return 'Last 7 Days';
    if (daysDiff === 29) return 'Last 30 Days';
    if (daysDiff === 89) return 'Last 90 Days';
  }

  // Default to showing the date range
  return `${startDate} - ${endDate}`;
}

// Parse filters from URL search params
function parseFiltersFromURL(searchParams: URLSearchParams): TrendsFiltersValue | null {
  const start = searchParams.get('start');
  const end = searchParams.get('end');

  // If no date params, return null to use defaults
  if (!start || !end) return null;

  // Validate date format (YYYY-MM-DD)
  const dateRegex = /^\d{4}-\d{2}-\d{2}$/;
  if (!dateRegex.test(start) || !dateRegex.test(end)) return null;

  // Use getAll for repos to support multiple 'repo' params
  const repos = searchParams.getAll('repo').filter(Boolean);
  const includeNoRepo = searchParams.get('includeNoRepo');

  return {
    dateRange: {
      startDate: start,
      endDate: end,
      label: getDateRangeLabel(start, end),
    },
    repos,
    includeNoRepo: includeNoRepo !== 'false', // Default to true
  };
}

// Serialize filters to URL search params
function serializeFiltersToURL(filters: TrendsFiltersValue): URLSearchParams {
  const params = new URLSearchParams();
  params.set('start', filters.dateRange.startDate);
  params.set('end', filters.dateRange.endDate);

  // Use multiple 'repo' params instead of comma-separated to handle URLs with commas
  filters.repos.forEach((repo) => {
    params.append('repo', repo);
  });

  if (!filters.includeNoRepo) {
    params.set('includeNoRepo', 'false');
  }

  return params;
}

function TrendsPage() {
  useDocumentTitle('Personal Trends');
  const [searchParams, setSearchParams] = useSearchParams();

  // Parse initial filters from URL or use defaults
  const initialFilters = useMemo(() => {
    return parseFiltersFromURL(searchParams) ?? {
      dateRange: getDefaultDateRange(),
      repos: [],
      includeNoRepo: true,
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Only run once on mount

  // Get repos from sessions list API (uses filter_options which covers ALL visible sessions, not just page 1)
  const [repos, setRepos] = useState<string[]>([]);
  useEffect(() => {
    sessionsAPI.list().then((result) => {
      setRepos(result.filter_options.repos.sort());
    }).catch(() => {
      // Silently fail - repos dropdown will just be empty
    });
  }, []);

  // Filter state
  const [filters, setFilters] = useState<TrendsFiltersValue>(initialFilters);

  // Track if initial URL had explicit repo params (to know if we should auto-select all)
  const hadExplicitRepoParams = useRef(searchParams.getAll('repo').length > 0);
  const hasAutoSelectedRepos = useRef(false);

  // Fetch trends data
  const { data, loading, error, refetch } = useTrends({
    startDate: filters.dateRange.startDate,
    endDate: filters.dateRange.endDate,
    repos: filters.repos,
    includeNoRepo: filters.includeNoRepo,
  });

  // Ref to access refetch without adding to useEffect deps
  const refetchRef = useRef(refetch);
  useEffect(() => {
    refetchRef.current = refetch;
  }, [refetch]);

  // Auto-select all repos on initial load if no explicit repo params in URL
  useEffect(() => {
    if (hasAutoSelectedRepos.current) return;
    if (repos.length === 0) return;
    if (hadExplicitRepoParams.current) return; // User had explicit repos in URL

    hasAutoSelectedRepos.current = true;
    setFilters(prev => {
      const newFilters = { ...prev, repos: [...repos] };
      // Refetch with the new repos
      refetchRef.current({
        startDate: newFilters.dateRange.startDate,
        endDate: newFilters.dateRange.endDate,
        repos: newFilters.repos,
        includeNoRepo: newFilters.includeNoRepo,
      });
      return newFilters;
    });
  }, [repos]);

  // Update URL when filters change
  useEffect(() => {
    const newParams = serializeFiltersToURL(filters);
    setSearchParams(newParams, { replace: true });
  }, [filters, setSearchParams]);

  // Handle filter changes
  const handleFilterChange = useCallback((newFilters: TrendsFiltersValue) => {
    setFilters(newFilters);
    refetch({
      startDate: newFilters.dateRange.startDate,
      endDate: newFilters.dateRange.endDate,
      repos: newFilters.repos,
      includeNoRepo: newFilters.includeNoRepo,
    });
  }, [refetch]);

  const showEmptyState = !loading && data && data.session_count === 0;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Personal Trends</h1>}
          actions={
            <TrendsFilters
              repos={repos}
              value={filters}
              onChange={handleFilterChange}
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
              <TrendsAgentsAndSkillsCard data={data.cards.agents_and_skills} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default TrendsPage;
