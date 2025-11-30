import { useState, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessions, useDocumentTitle, useSuccessMessage } from '@/hooks';
import { formatRelativeTime, sortData, type SortDirection } from '@/utils';
import PageHeader from '@/components/PageHeader';
import PageSidebar, { SidebarItem } from '@/components/PageSidebar';
import SortableHeader from '@/components/SortableHeader';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import styles from './SessionsPage.module.css';

type SortColumn = 'title' | 'external_id' | 'last_sync_time';

// SVG Icons
const RepoIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
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
  const [showSharedWithMe, setShowSharedWithMe] = useState(false);
  const { sessions, loading, error } = useSessions(showSharedWithMe);
  const [sortColumn, setSortColumn] = useState<SortColumn>('last_sync_time');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const { message: successMessage, fading: successFading } = useSuccessMessage();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [selectedRepo, setSelectedRepo] = useState<string | null>(null);
  const [selectedBranch, setSelectedBranch] = useState<string | null>(null);

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortDirection(column === 'last_sync_time' ? 'desc' : 'asc');
    }
  };

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

  const handleRepoClick = (repo: string | null) => {
    setSelectedRepo(repo);
    setSelectedBranch(null); // Reset branch filter when repo changes
  };

  return (
    <div className={styles.pageWrapper}>
      <PageSidebar
        title="Filters"
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
      >
        {/* All Sessions */}
        <SidebarItem
          icon={AllIcon}
          label="All Sessions"
          count={totalCount}
          active={!selectedRepo && !selectedBranch}
          onClick={() => {
            setSelectedRepo(null);
            setSelectedBranch(null);
          }}
          collapsed={sidebarCollapsed}
        />

        {/* Repos section */}
        {repos.length > 0 && (
          <>
            {!sidebarCollapsed && <div className={styles.sidebarDivider} />}
            {repos.map((repo) => (
              <SidebarItem
                key={repo}
                icon={RepoIcon}
                label={repo}
                count={repoCounts[repo] || 0}
                active={selectedRepo === repo}
                onClick={() => handleRepoClick(repo)}
                collapsed={sidebarCollapsed}
              />
            ))}
          </>
        )}

        {/* Branches section - only show when a repo is selected */}
        {selectedRepo && branches.length > 0 && (
          <>
            {!sidebarCollapsed && <div className={styles.sidebarDivider} />}
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
                  collapsed={sidebarCollapsed}
                />
              ))}
          </>
        )}
      </PageSidebar>

      <div className={`${styles.mainContent} ${sidebarCollapsed ? styles.sidebarCollapsed : ''}`}>
        <PageHeader
          title={showSharedWithMe ? 'Shared with me' : 'Sessions'}
          subtitle={`${sortedSessions.length} session${sortedSessions.length !== 1 ? 's' : ''}`}
          actions={
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
