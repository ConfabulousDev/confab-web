import { useState, useMemo, useCallback, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useDocumentTitle, useOrgAnalytics } from '@/hooks';
import { getDefaultDateRange, parseDateRangeFromURL } from '@/utils';
import PageHeader from '@/components/PageHeader';
import OrgFilters, { type OrgFiltersValue } from '@/components/org/OrgFilters';
import OrgTable from '@/components/org/OrgTable';
import Alert from '@/components/Alert';
import styles from './OrgPage.module.css';

function parseFiltersFromURL(searchParams: URLSearchParams): OrgFiltersValue | null {
  const dateRange = parseDateRangeFromURL(searchParams);
  if (!dateRange) return null;
  return { dateRange };
}

function serializeFiltersToURL(filters: OrgFiltersValue): URLSearchParams {
  const params = new URLSearchParams();
  params.set('start', filters.dateRange.startDate);
  params.set('end', filters.dateRange.endDate);
  return params;
}

function OrgPage() {
  useDocumentTitle('Organization');
  const [searchParams, setSearchParams] = useSearchParams();

  const initialFilters = useMemo(() => {
    return parseFiltersFromURL(searchParams) ?? {
      dateRange: getDefaultDateRange(),
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [filters, setFilters] = useState<OrgFiltersValue>(initialFilters);

  const { data, loading, error, refetch } = useOrgAnalytics({
    startDate: filters.dateRange.startDate,
    endDate: filters.dateRange.endDate,
  });

  useEffect(() => {
    const newParams = serializeFiltersToURL(filters);
    setSearchParams(newParams, { replace: true });
  }, [filters, setSearchParams]);

  const handleFilterChange = useCallback((newFilters: OrgFiltersValue) => {
    setFilters(newFilters);
    refetch({
      startDate: newFilters.dateRange.startDate,
      endDate: newFilters.dateRange.endDate,
    });
  }, [refetch]);

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

          {hasData && <OrgTable users={data.users} />}
        </div>
      </div>
    </div>
  );
}

export default OrgPage;
