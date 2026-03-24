import { useCallback } from 'react';
import { useDocumentTitle, useOrgAnalytics, useURLFilters } from '@/hooks';
import type { URLFiltersConfig } from '@/hooks';
import { getDefaultDateRange } from '@/utils';
import PageHeader from '@/components/PageHeader';
import OrgFilters, { type OrgFiltersValue } from '@/components/org/OrgFilters';
import OrgTable from '@/components/org/OrgTable';
import Alert from '@/components/Alert';
import styles from './OrgPage.module.css';

function OrgPage() {
  useDocumentTitle('Organization');

  const config: URLFiltersConfig = {
    dateRange: { type: 'dateRange', default: getDefaultDateRange(), paramName: { start: 'start', end: 'end' } },
  };

  const { filters, setFilter } = useURLFilters<OrgFiltersValue>(config);

  const { data, loading, error, refetch } = useOrgAnalytics({
    startDate: filters.dateRange.startDate,
    endDate: filters.dateRange.endDate,
  });

  const handleFilterChange = useCallback((newFilters: OrgFiltersValue) => {
    setFilter('dateRange', newFilters.dateRange);
    refetch({
      startDate: newFilters.dateRange.startDate,
      endDate: newFilters.dateRange.endDate,
    });
  }, [setFilter, refetch]);

  const showEmpty = !loading && data && data.users.every(u => u.session_count === 0);
  const hasData = !loading && data && data.users.length > 0 && !showEmpty;

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>Organization</h1>}
          actions={
            <OrgFilters
              value={filters}
              onChange={handleFilterChange}
            />
          }
        />

        <div className={styles.container}>
          {error && <Alert variant="error">{error.message}</Alert>}

          {loading && !data && (
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
