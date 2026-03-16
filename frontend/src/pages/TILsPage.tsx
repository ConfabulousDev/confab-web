import { useRef, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTILsFetch, useAuth, useDocumentTitle, useSessionFilters, useColumnCount, distributeToColumns } from '@/hooks';
import type { TILWithSession } from '@/schemas/api';
import PageHeader from '@/components/PageHeader';
import FilterChipsBar from '@/components/FilterChipsBar';
import Pagination from '@/components/Pagination';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import TILCard from '@/components/TILCard';
import { RefreshIcon } from '@/components/icons';
import styles from './TILsPage.module.css';

function TILsPage() {
  useDocumentTitle('TILs');
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const {
    repos, branches, owners, query,
    toggleRepo, toggleBranch, toggleOwner,
    setQuery, clearAll,
  } = useSessionFilters();

  const { tils, hasMore, filterOptions, loading, error, refetch, goNext, goPrev, canGoPrev, deleteTIL } = useTILsFetch({
    repos, branches, owners, query,
  });
  const { user } = useAuth();
  const columnCount = useColumnCount();

  const ownersExceptSelf = owners.filter((o) => o !== user?.email);
  const hasActiveFilters = repos.length > 0 || branches.length > 0 || ownersExceptSelf.length > 0 || query !== '';

  const columns = useMemo(
    () => distributeToColumns(tils, columnCount),
    [tils, columnCount],
  );

  const handleNavigate = (til: TILWithSession) => {
    let url = `/sessions/${til.session_id}?tab=transcript`;
    if (til.message_uuid) {
      url += `&msg=${til.message_uuid}`;
    }
    navigate(url);
  };

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={<h1 className={styles.title}>TILs</h1>}
          actions={
            <div className={styles.toolbarActions}>
              <Pagination
                hasMore={hasMore}
                canGoPrev={canGoPrev}
                onNext={goNext}
                onPrev={goPrev}
              />
              <button
                className={styles.refreshBtn}
                onClick={() => refetch()}
                title="Refresh TILs"
                aria-label="Refresh TILs"
                disabled={loading}
              >
                {RefreshIcon}
              </button>
            </div>
          }
        />
        <div className={styles.filterBar}>
          <FilterChipsBar
            filters={{ repos, branches, owners, query }}
            filterOptions={filterOptions}
            currentUserEmail={user?.email ?? null}
            onToggleRepo={toggleRepo}
            onToggleBranch={toggleBranch}
            onToggleOwner={toggleOwner}
            onQueryChange={setQuery}
            onClearAll={clearAll}
          />
        </div>

        <div ref={containerRef} className={styles.container}>
          <ScrollNavButtons scrollRef={containerRef} />

          {error && <Alert variant="error">{error.message}</Alert>}

          {loading && tils.length === 0 && (
            <p className={styles.loading}>Loading TILs...</p>
          )}

          {!loading && tils.length === 0 && (
            <div className={styles.emptyState}>
              {hasActiveFilters ? (
                'No TILs match your filters.'
              ) : (
                <>No TILs yet. Use <code>/til</code> in Claude Code to save learnings from your sessions.</>
              )}
            </div>
          )}

          {tils.length > 0 && (
            <div className={`${styles.masonry} ${loading ? styles.masonryLoading : ''}`}>
              {columns.map((colTils, colIndex) => (
                <div key={colIndex} className={styles.column}>
                  {colTils.map((til) => (
                    <TILCard
                      key={til.id}
                      til={til}
                      onNavigate={() => handleNavigate(til)}
                      onDelete={() => deleteTIL(til.id)}
                    />
                  ))}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default TILsPage;
