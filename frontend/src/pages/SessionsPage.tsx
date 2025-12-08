import { useCallback, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessions, useDocumentTitle, useSuccessMessage, useSessionFilters } from '@/hooks';
import { useKeyboardShortcut } from '@/hooks/useKeyboardShortcut';
import { formatRelativeTime, sortData } from '@/utils';
import PageHeader from '@/components/PageHeader';
import SessionListStatsSidebar from '@/components/SessionListStatsSidebar';
import SessionsFilterDropdown from '@/components/SessionsFilterDropdown';
import SortableHeader from '@/components/SortableHeader';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import styles from './SessionsPage.module.css';

function SessionsPage() {
  useDocumentTitle('Sessions');
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const {
    showSharedWithMe,
    setShowSharedWithMe,
    selectedRepo,
    selectedBranch,
    setSelectedBranch,
    sortColumn,
    sortDirection,
    handleSort,
    handleRepoClick,
    showEmptySessions,
    toggleShowEmptySessions,
  } = useSessionFilters();

  // Hidden keyboard shortcut to toggle showing empty sessions (for dev/debugging)
  const handleToggleEmptySessions = useCallback(() => {
    toggleShowEmptySessions();
  }, [toggleShowEmptySessions]);
  useKeyboardShortcut('mod+shift+e', handleToggleEmptySessions);
  const { sessions, loading, error } = useSessions(showSharedWithMe);
  const { message: successMessage, fading: successFading } = useSuccessMessage();

  // Get unique repos and branches for filtering
  const { repos, branches } = useMemo(() => {
    const repoSet = new Set<string>();
    const branchSet = new Set<string>();

    sessions.forEach((s) => {
      if (s.git_repo) repoSet.add(s.git_repo);
      if (s.git_branch) branchSet.add(s.git_branch);
    });

    return {
      repos: Array.from(repoSet).sort(),
      branches: Array.from(branchSet).sort(),
    };
  }, [sessions]);

  // Sorted and filtered sessions
  const sortedSessions = useMemo(() => {
    return sortData({
      data: sessions,
      sortBy: sortColumn,
      direction: sortDirection,
      filter: (s) => {
        // Filter out sessions with no transcript data
        if (s.total_lines <= 0) return false;
        // Filter out empty sessions (no title) unless showEmptySessions is enabled
        if (!showEmptySessions && !s.summary && !s.first_user_message) return false;
        // Show only owned or only shared based on toggle
        if (showSharedWithMe ? s.is_owner : !s.is_owner) return false;
        // Filter by selected repo
        if (selectedRepo && s.git_repo !== selectedRepo) return false;
        // Filter by selected branch
        if (selectedBranch && s.git_branch !== selectedBranch) return false;
        return true;
      },
    });
  }, [sessions, sortColumn, sortDirection, showSharedWithMe, selectedRepo, selectedBranch, showEmptySessions]);

  // Helper to check if a session passes the base filters (excluding repo/branch)
  const passesBaseFilters = useCallback((s: typeof sessions[0]) => {
    if (s.total_lines <= 0) return false;
    if (!showEmptySessions && !s.summary && !s.first_user_message) return false;
    if (showSharedWithMe ? s.is_owner : !s.is_owner) return false;
    return true;
  }, [showSharedWithMe, showEmptySessions]);

  // Count sessions per repo/branch
  const repoCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (passesBaseFilters(s)) {
        const repo = s.git_repo || '';
        counts[repo] = (counts[repo] || 0) + 1;
      }
    });
    return counts;
  }, [sessions, passesBaseFilters]);

  const branchCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (passesBaseFilters(s)) {
        if (!selectedRepo || s.git_repo === selectedRepo) {
          const branch = s.git_branch || '';
          counts[branch] = (counts[branch] || 0) + 1;
        }
      }
    });
    return counts;
  }, [sessions, passesBaseFilters, selectedRepo]);

  const totalCount = useMemo(() => {
    return sessions.filter(passesBaseFilters).length;
  }, [sessions, passesBaseFilters]);

  const handleRowClick = (session: typeof sessions[0]) => {
    if (session.is_owner) {
      navigate(`/sessions/${session.id}`);
    } else {
      navigate(`/sessions/${session.id}/shared/${session.share_token}`);
    }
  };

  return (
    <div className={styles.pageWrapper}>
      <SessionListStatsSidebar sessions={sortedSessions} loading={loading} />

      <div className={styles.mainContent}>
        <PageHeader
          leftContent={
            <>
              <div className={styles.tabs}>
                <button
                  className={`${styles.tab} ${!showSharedWithMe ? styles.active : ''}`}
                  onClick={() => setShowSharedWithMe(false)}
                >
                  Mine
                </button>
                <button
                  className={`${styles.tab} ${showSharedWithMe ? styles.active : ''}`}
                  onClick={() => setShowSharedWithMe(true)}
                >
                  Shared with me
                </button>
              </div>
              <span className={styles.sessionCount}>
                  {sortedSessions.length} session{sortedSessions.length !== 1 ? 's' : ''}
                  {showEmptySessions && <span className={styles.devIndicator}> (showing empty)</span>}
                </span>
            </>
          }
          actions={
            <SessionsFilterDropdown
              repos={repos}
              branches={branches}
              selectedRepo={selectedRepo}
              selectedBranch={selectedBranch}
              repoCounts={repoCounts}
              branchCounts={branchCounts}
              totalCount={totalCount}
              onRepoClick={handleRepoClick}
              onBranchClick={(branch) => setSelectedBranch(branch)}
            />
          }
        />

        <div ref={containerRef} className={styles.container}>
          <ScrollNavButtons scrollRef={containerRef} />

          {successMessage && (
            <Alert
              variant="success"
              className={`${styles.successAlert} ${successFading ? styles.alertFading : ''}`}
            >
              {successMessage}
            </Alert>
          )}
          {error && <Alert variant="error">{error}</Alert>}

          <div className={styles.card}>
            {loading ? (
              <p className={styles.loading}>Loading sessions...</p>
            ) : sortedSessions.length === 0 ? (
              <p className={styles.empty}>
                {showSharedWithMe
                  ? 'No sessions have been shared with you yet.'
                  : selectedRepo || selectedBranch
                    ? 'No sessions match the selected filters.'
                    : 'No sessions yet. Sessions will appear here after you use confab.'}
              </p>
            ) : (
              <div className={styles.sessionsTable}>
                <table>
                  <thead>
                    <tr>
                      <SortableHeader
                        column="summary"
                        label="Title"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                      <th>Repo</th>
                      <th>Branch</th>
                      <SortableHeader
                        column="external_id"
                        label="ID"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                      <SortableHeader
                        column="last_sync_time"
                        label="Activity"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                    </tr>
                  </thead>
                  <tbody>
                    {sortedSessions.map((session) => (
                      <tr
                        key={session.id}
                        className={styles.clickableRow}
                        onClick={() => handleRowClick(session)}
                      >
                        <td className={styles.titleCell}>
                          <span className={session.summary || session.first_user_message ? '' : styles.untitled}>
                            {session.summary || session.first_user_message || 'Untitled'}
                          </span>
                        </td>
                        <td className={styles.gitInfo}>{session.git_repo || ''}</td>
                        <td className={styles.gitInfo}>{session.git_branch || ''}</td>
                        <td className={styles.sessionId}>{session.external_id.substring(0, 8)}</td>
                        <td className={styles.timestamp}>{session.last_sync_time ? formatRelativeTime(session.last_sync_time) : '-'}</td>
                      </tr>
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

export default SessionsPage;
