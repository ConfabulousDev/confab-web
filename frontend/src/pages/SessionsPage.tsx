import { useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessions, useDocumentTitle, useSuccessMessage, useSessionFilters } from '@/hooks';
import { formatRelativeTime, sortData } from '@/utils';
import PageHeader from '@/components/PageHeader';
import PageSidebar, { SidebarItem } from '@/components/PageSidebar';
import SortableHeader from '@/components/SortableHeader';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import styles from './SessionsPage.module.css';

// SVG Icons
const GitHubIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
  </svg>
);

const BranchIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="6" y1="3" x2="6" y2="15" />
    <circle cx="18" cy="6" r="3" />
    <circle cx="6" cy="18" r="3" />
    <path d="M18 9a9 9 0 0 1-9 9" />
  </svg>
);

const AllIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="3" y="3" width="7" height="7" />
    <rect x="14" y="3" width="7" height="7" />
    <rect x="14" y="14" width="7" height="7" />
    <rect x="3" y="14" width="7" height="7" />
  </svg>
);

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
  } = useSessionFilters();
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
        // Filter out empty sessions
        if (s.total_lines <= 0) return false;
        // Show only owned or only shared based on toggle
        if (showSharedWithMe ? s.is_owner : !s.is_owner) return false;
        // Filter by selected repo
        if (selectedRepo && s.git_repo !== selectedRepo) return false;
        // Filter by selected branch
        if (selectedBranch && s.git_branch !== selectedBranch) return false;
        return true;
      },
    });
  }, [sessions, sortColumn, sortDirection, showSharedWithMe, selectedRepo, selectedBranch]);

  // Count sessions per repo/branch
  const repoCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (s.total_lines > 0 && (showSharedWithMe ? !s.is_owner : s.is_owner)) {
        const repo = s.git_repo || '';
        counts[repo] = (counts[repo] || 0) + 1;
      }
    });
    return counts;
  }, [sessions, showSharedWithMe]);

  const branchCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (s.total_lines > 0 && (showSharedWithMe ? !s.is_owner : s.is_owner)) {
        if (!selectedRepo || s.git_repo === selectedRepo) {
          const branch = s.git_branch || '';
          counts[branch] = (counts[branch] || 0) + 1;
        }
      }
    });
    return counts;
  }, [sessions, showSharedWithMe, selectedRepo]);

  const totalCount = useMemo(() => {
    return sessions.filter((s) =>
      s.total_lines > 0 && (showSharedWithMe ? !s.is_owner : s.is_owner)
    ).length;
  }, [sessions, showSharedWithMe]);

  const handleRowClick = (session: typeof sessions[0]) => {
    if (session.is_owner) {
      navigate(`/sessions/${session.id}`);
    } else {
      navigate(`/sessions/${session.id}/shared/${session.share_token}`);
    }
  };

  return (
    <div className={styles.pageWrapper}>
      <PageSidebar collapsible={false}>
        {/* All Sessions */}
        <SidebarItem
          icon={AllIcon}
          label="All Sessions"
          count={totalCount}
          active={!selectedRepo && !selectedBranch}
          onClick={() => handleRepoClick(null)}
        />

        {/* Repos section */}
        {repos.length > 0 && (
          <>
            <div className={styles.sidebarDivider} />
            {repos.map((repo) => (
              <SidebarItem
                key={repo}
                icon={GitHubIcon}
                label={repo}
                count={repoCounts[repo] || 0}
                active={selectedRepo === repo}
                onClick={() => handleRepoClick(repo)}
              />
            ))}
          </>
        )}

        {/* Branches section - only show when a repo is selected */}
        {selectedRepo && branches.length > 0 && (
          <>
            <div className={styles.sidebarDivider} />
            {branches
              .filter((branch) => (branchCounts[branch] ?? 0) > 0)
              .map((branch) => (
                <SidebarItem
                  key={branch}
                  icon={BranchIcon}
                  label={branch}
                  count={branchCounts[branch] || 0}
                  active={selectedBranch === branch}
                  onClick={() => setSelectedBranch(selectedBranch === branch ? null : branch)}
                />
              ))}
          </>
        )}
      </PageSidebar>

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
              <span className={styles.sessionCount}>{sortedSessions.length} session{sortedSessions.length !== 1 ? 's' : ''}</span>
            </>
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
                        column="title"
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
                          <span className={session.title ? '' : styles.untitled}>
                            {session.title || 'Untitled'}
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
