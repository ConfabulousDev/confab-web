import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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

  // `availableRepos === null` means the /org/repos call is still in flight (or
  // failed); `[]` means "resolved, no repos in range". We MUST distinguish the
  // two: firing useOrgAnalytics with the URL's default `repos: []` before
  // /org/repos lands would render no-repo-only data for a beat, then flip to
  // correct data once auto-select-all kicks in. Reviewer flagged the flicker
  // + the wasted query.
  const [availableRepos, setAvailableRepos] = useState<string[] | null>(null);
  const [reposError, setReposError] = useState<Error | null>(null);
  useEffect(() => {
    // Note: not clearing prior state synchronously here — the lint rule flags
    // it, and the resolve/catch handlers overwrite both. The visible effect is
    // that the repo dropdown holds the previous range's list during refetch,
    // which is fine (TrendsPage behaves the same way).
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

  // Captured once at mount: "did the URL pin explicit repo params?" Drives the
  // auto-select-all-on-load decision below. useState (not useRef) so we can
  // read it during render to gate `ready` without tripping React's ref rules.
  const [hadExplicitRepoParams] = useState(() => filters.repos.length > 0);
  const hasAutoSelectedRepos = useRef(false);
  useEffect(() => {
    if (hasAutoSelectedRepos.current) return;
    if (availableRepos === null) return;
    if (hadExplicitRepoParams) {
      hasAutoSelectedRepos.current = true;
      return;
    }
    hasAutoSelectedRepos.current = true;
    if (availableRepos.length > 0) {
      setAll({ repos: [...availableRepos] }, { replace: true });
    }
  }, [availableRepos, hadExplicitRepoParams, setAll]);

  // Hold the data fetch until either (a) /org/repos has resolved and (if
  // auto-selecting) repos have been written to the URL, or (b) the user
  // landed with explicit `?repo=` params we can honor immediately.
  const ready = hadExplicitRepoParams
    ? true
    : availableRepos !== null && (availableRepos.length === 0 || filters.repos.length > 0);

  const { data, loading, error, refetch } = useOrgAnalytics(
    {
      startDate: filters.dateRange.startDate,
      endDate: filters.dateRange.endDate,
      providers: filters.providers,
      repos: filters.repos,
      includeNoRepo: filters.includeNoRepo,
    },
    { enabled: ready },
  );

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

  const isInitialLoading = !data && !error && (loading || !ready);
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
              availableRepos={availableRepos ?? []}
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
