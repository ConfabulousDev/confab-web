import { useRef, useState, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTILsFetch, useAuth, useDocumentTitle, useSessionFilters } from '@/hooks';
import type { TILWithSession } from '@/schemas/api';
import { RelativeTime } from '@/components/RelativeTime';
import FilterChipsBar from '@/components/FilterChipsBar';
import Pagination from '@/components/Pagination';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import Chip from '@/components/Chip';
import { RepoIcon, BranchIcon, PersonIcon, RefreshIcon } from '@/components/icons';
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

  const ownersExceptSelf = owners.filter((o) => o !== user?.email);
  const hasActiveFilters = repos.length > 0 || branches.length > 0 || ownersExceptSelf.length > 0 || query !== '';

  const handleRowClick = (sessionId: string, messageUuid?: string | null) => {
    let url = `/sessions/${sessionId}?tab=transcript`;
    if (messageUuid) {
      url += `&msg=${messageUuid}`;
    }
    navigate(url);
  };

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <header className={styles.toolbar}>
          <div className={styles.toolbarTop}>
            <span className={styles.pageTitle}>
              {loading && tils.length > 0 ? (
                <span className={styles.loadingIndicator}>Updating...</span>
              ) : (
                'TILs'
              )}
            </span>
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
          </div>

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
        </header>

        <div ref={containerRef} className={styles.container}>
          <ScrollNavButtons scrollRef={containerRef} />

          {error && <Alert variant="error">{error.message}</Alert>}

          <div className={styles.card}>
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
              <div className={`${styles.tilsTable} ${loading ? styles.tableLoading : ''}`}>
                <table>
                  <thead>
                    <tr>
                      <th>TIL</th>
                      <th>Created</th>
                      <th className={styles.actionsCell}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {tils.map((til) => (
                      <TILRow
                        key={til.id}
                        til={til}
                        onRowClick={() => handleRowClick(til.session_id, til.message_uuid)}
                        onDelete={() => deleteTIL(til.id)}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

interface TILRowProps {
  til: TILWithSession;
  onRowClick: () => void;
  onDelete: () => void;
}

function TILRow({ til, onRowClick, onDelete }: TILRowProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Clean up timer on unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleDelete = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirmDelete) {
      onDelete();
      setConfirmDelete(false);
      if (timerRef.current) clearTimeout(timerRef.current);
    } else {
      setConfirmDelete(true);
      timerRef.current = setTimeout(() => setConfirmDelete(false), 3000);
    }
  }, [confirmDelete, onDelete]);

  return (
    <tr className={styles.clickableRow} onClick={onRowClick}>
      <td className={styles.tilCell}>
        <div className={styles.tilTitle}>{til.title}</div>
        <div className={styles.tilSummary}>{til.summary}</div>
        <div className={styles.chipRow}>
          {til.session_title && (
            <Chip icon={null} variant="neutral">{til.session_title}</Chip>
          )}
          <Chip icon={PersonIcon} variant="neutral" copyValue={til.owner_email}>
            {til.owner_email}
          </Chip>
          {til.git_repo && (
            <Chip icon={RepoIcon} variant="neutral">{til.git_repo}</Chip>
          )}
          {til.git_branch && (
            <Chip icon={BranchIcon} variant="blue">{til.git_branch}</Chip>
          )}
        </div>
      </td>
      <td className={styles.timestamp}>
        <RelativeTime date={til.created_at} />
      </td>
      <td className={styles.actionsCell}>
        {til.is_owner && (
          <button
            className={confirmDelete ? styles.deleteBtnConfirm : styles.deleteBtn}
            onClick={handleDelete}
            title={confirmDelete ? 'Click again to confirm' : 'Delete TIL'}
          >
            {confirmDelete ? 'Confirm?' : 'Delete'}
          </button>
        )}
      </td>
    </tr>
  );
}

export default TILsPage;
