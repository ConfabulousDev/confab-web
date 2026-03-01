import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useDocumentTitle, useTrends } from '@/hooks';
import { sessionsAPI } from '@/services/api';
import { getDefaultDateRange, parseDateRangeFromURL } from '@/utils';
import PageHeader from '@/components/PageHeader';
import TrendsFilters, { type TrendsFiltersValue } from '@/components/trends/TrendsFilters';
import {
  TrendsOverviewCard,
  TrendsTokensCard,
  TrendsActivityCard,
  TrendsToolsCard,
  TrendsUtilizationCard,
  TrendsAgentsAndSkillsCard,
  TrendsTopSessionsCard,
} from '@/components/trends/cards';
import Alert from '@/components/Alert';
import CardGrid from '@/components/CardGrid';
import styles from './TrendsPage.module.css';

function parseFiltersFromURL(searchParams: URLSearchParams): TrendsFiltersValue | null {
  const dateRange = parseDateRangeFromURL(searchParams);
  if (!dateRange) return null;

  const repos = searchParams.getAll('repo').filter(Boolean);
  const includeNoRepo = searchParams.get('includeNoRepo');

  return {
    dateRange,
    repos,
    includeNoRepo: includeNoRepo !== 'false',
  };
}

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
            <CardGrid>
              <TrendsOverviewCard data={data.cards.overview} />
              <TrendsTokensCard data={data.cards.tokens} />
              <TrendsTopSessionsCard data={data.cards.top_sessions} />
              <TrendsActivityCard data={data.cards.activity} />
              <TrendsToolsCard data={data.cards.tools} />
              <TrendsUtilizationCard data={data.cards.utilization} />
              <TrendsAgentsAndSkillsCard data={data.cards.agents_and_skills} />
            </CardGrid>
          )}
        </div>
      </div>
    </div>
  );
}

export default TrendsPage;
