import { useState, useCallback, useEffect, useRef } from 'react';
import { useDocumentTitle, useTrends, useURLFilters } from '@/hooks';
import type { URLFiltersConfig } from '@/hooks';
import { sessionsAPI } from '@/services/api';
import { getDefaultDateRange } from '@/utils';
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

function TrendsPage() {
  useDocumentTitle('Personal Trends');

  // Config inside component so getDefaultDateRange() is fresh each render
  const config: URLFiltersConfig = {
    dateRange: { type: 'dateRange', default: getDefaultDateRange(), paramName: { start: 'start', end: 'end' } },
    repos: { type: 'string[]', default: [], paramName: 'repo' },
    includeNoRepo: { type: 'boolean', default: true, paramName: 'includeNoRepo' },
    // CF-424: singular `provider` wire key matches the session-list endpoint.
    // Empty default = "all providers"; we deliberately do not auto-select-all
    // like repos so the URL stays clean for the common case.
    providers: { type: 'string[]', default: [], paramName: 'provider' },
  };

  const { filters, setAll } = useURLFilters<TrendsFiltersValue>(config);

  // Get repos from sessions list API
  const [availableRepos, setAvailableRepos] = useState<string[]>([]);
  useEffect(() => {
    sessionsAPI.list().then((result) => {
      setAvailableRepos(result.filter_options.repos.sort());
    }).catch(() => {
      // Silently fail - repos dropdown will just be empty
    });
  }, []);

  // Auto-select all repos on initial load if no explicit repo params in URL
  const hadExplicitRepoParams = useRef(filters.repos.length > 0);
  const hasAutoSelectedRepos = useRef(false);

  // Fetch trends data
  const { data, loading, error, refetch } = useTrends({
    startDate: filters.dateRange.startDate,
    endDate: filters.dateRange.endDate,
    repos: filters.repos,
    includeNoRepo: filters.includeNoRepo,
    providers: filters.providers,
  });

  const refetchRef = useRef(refetch);
  useEffect(() => {
    refetchRef.current = refetch;
  }, [refetch]);

  // Auto-select all repos on initial load if no explicit repo params in URL
  useEffect(() => {
    if (hasAutoSelectedRepos.current) return;
    if (availableRepos.length === 0) return;
    if (hadExplicitRepoParams.current) return;

    hasAutoSelectedRepos.current = true;
    const newRepos = [...availableRepos];
    setAll({ repos: newRepos }, { replace: true });
    refetchRef.current({
      startDate: filters.dateRange.startDate,
      endDate: filters.dateRange.endDate,
      repos: newRepos,
      includeNoRepo: filters.includeNoRepo,
      providers: filters.providers,
    });
  }, [availableRepos, filters.dateRange, filters.includeNoRepo, filters.providers, setAll]);

  // Handle filter changes from TrendsFilters component
  const handleFilterChange = useCallback((newFilters: TrendsFiltersValue) => {
    setAll(newFilters);
    refetch({
      startDate: newFilters.dateRange.startDate,
      endDate: newFilters.dateRange.endDate,
      repos: newFilters.repos,
      includeNoRepo: newFilters.includeNoRepo,
      providers: newFilters.providers,
    });
  }, [setAll, refetch]);

  const showEmptyState = !loading && data && data.session_count === 0;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Personal Trends</h1>}
          actions={
            <TrendsFilters
              repos={availableRepos}
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
                No sessions match the selected filters. Try adjusting the date range, repo filter, or provider filter.
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
