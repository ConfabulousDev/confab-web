import { useCallback, useMemo } from 'react';
import { useAuth, useDocumentTitle, useTrends, useURLFilters } from '@/hooks';
import type { URLFiltersConfig } from '@/hooks';
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
  useDocumentTitle('Trends');
  const { user } = useAuth();

  // Config inside component so getDefaultDateRange() is fresh each render
  const config: URLFiltersConfig = {
    dateRange: { type: 'dateRange', default: getDefaultDateRange(), paramName: { start: 'start', end: 'end' } },
    repos: { type: 'string[]', default: [], paramName: 'repo' },
    includeNoRepo: { type: 'boolean', default: true, paramName: 'includeNoRepo' },
    // CF-424: singular `provider` wire key matches the session-list endpoint.
    // Empty default = "all providers"; we deliberately do not auto-select-all
    // like repos so the URL stays clean for the common case.
    providers: { type: 'string[]', default: [], paramName: 'provider' },
    // CF-495: owner narrows within visible set. Empty = "all owners"; same
    // semantics as providers. URL uses singular `owner` key matching Sessions.
    owners: { type: 'string[]', default: [], paramName: 'owner' },
  };

  const { filters, setAll } = useURLFilters<TrendsFiltersValue>(config);

  // Fetch trends data. CF-495: filter_options.repos + .owners come from the
  // response itself — no side-call to /api/sessions needed.
  const { data, loading, error, refetch } = useTrends({
    startDate: filters.dateRange.startDate,
    endDate: filters.dateRange.endDate,
    repos: filters.repos,
    includeNoRepo: filters.includeNoRepo,
    providers: filters.providers,
    owners: filters.owners,
  });

  const availableRepos = useMemo(() => data?.filter_options.repos ?? [], [data]);
  const availableOwners = useMemo(() => data?.filter_options.owners ?? [], [data]);

  const handleFilterChange = useCallback((newFilters: TrendsFiltersValue) => {
    setAll(newFilters);
    refetch({
      startDate: newFilters.dateRange.startDate,
      endDate: newFilters.dateRange.endDate,
      repos: newFilters.repos,
      includeNoRepo: newFilters.includeNoRepo,
      providers: newFilters.providers,
      owners: newFilters.owners,
    });
  }, [setAll, refetch]);

  // CF-495: owner-narrowed empty state — when a filter is set but yields
  // zero sessions, hint at the cause and offer a one-click clear.
  const clearOwnerFilter = useCallback(() => {
    handleFilterChange({ ...filters, owners: [] });
  }, [filters, handleFilterChange]);

  const showEmptyState = !loading && data && data.session_count === 0;
  const ownerNarrowedEmpty = showEmptyState && filters.owners.length > 0;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Trends</h1>}
          actions={
            <TrendsFilters
              repos={availableRepos}
              owners={availableOwners}
              selfEmail={user?.email}
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

          {showEmptyState && ownerNarrowedEmpty && (
            <div className={styles.emptyState}>
              <div className={styles.emptyStateTitle}>No sessions match the owner filter</div>
              <div className={styles.emptyStateText}>
                Try a different owner, or clear the filter to aggregate across all visible sessions.
              </div>
              <button className={styles.clearFilterBtn} onClick={clearOwnerFilter}>
                Clear owner filter
              </button>
            </div>
          )}

          {showEmptyState && !ownerNarrowedEmpty && (
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
              <TrendsActivityCard data={data.cards.activity} providersPresent={data.providers_present} />
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
