import { useCallback, useEffect, useMemo, useState } from 'react';
import { useDocumentTitle, useOrgAnalytics, useURLFilters } from '@/hooks';
import type { URLFiltersConfig } from '@/hooks';
import { orgReposAPI } from '@/services/api';
import { getDefaultDateRange } from '@/utils';
import { PROVIDER_VALUES } from '@/utils/providers';
import PageHeader from '@/components/PageHeader';
import OrgFilters, { type OrgFiltersValue } from '@/components/org/OrgFilters';
import OrgTable from '@/components/org/OrgTable';
import Alert from '@/components/Alert';
import styles from './OrgPage.module.css';

function OrgPage() {
  useDocumentTitle('Organization');

  const config: URLFiltersConfig = {
    dateRange: { type: 'dateRange', default: getDefaultDateRange(), paramName: { start: 'start', end: 'end' } },
    providers: { type: 'string[]', default: [], paramName: 'provider' },
    repos: { type: 'string[]', default: [], paramName: 'repo' },
    includeNoRepo: { type: 'boolean', default: true, paramName: 'includeNoRepo' },
  };

  const { filters, setAll } = useURLFilters<OrgFiltersValue>(config);

  const [availableRepos, setAvailableRepos] = useState<string[]>([]);
  const [reposError, setReposError] = useState<Error | null>(null);
  useEffect(() => {
    let cancelled = false;
    orgReposAPI
      .get({ startDate: filters.dateRange.startDate, endDate: filters.dateRange.endDate })
      .then((result) => {
        if (cancelled) return;
        setAvailableRepos(result.repos);
        setReposError(null);
      })
      .catch((err) => {
        if (cancelled) return;
        setAvailableRepos([]);
        setReposError(err instanceof Error ? err : new Error('Failed to load org repos'));
      });
    return () => {
      cancelled = true;
    };
  }, [filters.dateRange.startDate, filters.dateRange.endDate]);

  const { data, loading, error, refetch } = useOrgAnalytics({
    startDate: filters.dateRange.startDate,
    endDate: filters.dateRange.endDate,
    providers: filters.providers,
    repos: filters.repos,
    includeNoRepo: filters.includeNoRepo,
  });

  // Narrow the provider dropdown to providers with data in range once the
  // first response lands; before then, offer the full canonical list so the
  // dropdown is usable on initial paint.
  const availableProviders = useMemo<string[]>(() => {
    if (data && data.providers_present.length > 0) return data.providers_present;
    return [...PROVIDER_VALUES];
  }, [data]);

  const handleFilterChange = useCallback((newFilters: OrgFiltersValue) => {
    setAll(newFilters);
    refetch({
      startDate: newFilters.dateRange.startDate,
      endDate: newFilters.dateRange.endDate,
      providers: newFilters.providers,
      repos: newFilters.repos,
      includeNoRepo: newFilters.includeNoRepo,
    });
  }, [setAll, refetch]);

  const isInitialLoading = !data && !error && loading;
  const showEmpty = !loading && data && data.users.every(u => u.session_count === 0);
  const hasData = !loading && data && data.users.length > 0 && !showEmpty;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Organization</h1>}
          actions={
            <OrgFilters
              availableProviders={availableProviders}
              availableRepos={availableRepos}
              value={filters}
              onChange={handleFilterChange}
            />
          }
        />

        <div className={styles.container}>
          {reposError && <Alert variant="error">{reposError.message}</Alert>}
          {error && <Alert variant="error">{error.message}</Alert>}

          {isInitialLoading && (
            <div className={styles.loading}>Loading organization analytics...</div>
          )}

          {showEmpty && (
            <div className={styles.emptyState}>
              <div className={styles.emptyStateTitle}>No session data available</div>
              <div className={styles.emptyStateText}>
                No session data available for this period. Sessions appear here once analytics have been computed.
              </div>
            </div>
          )}

          {hasData && (
            <>
              <div className={styles.caption}>
                Averages are per-session. Costs are estimates based on token usage.
              </div>
              <OrgTable users={data.users} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

export default OrgPage;
