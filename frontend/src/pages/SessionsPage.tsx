import { useCallback, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessionsPolling, useDocumentTitle, useSuccessMessage, useSessionFilters } from '@/hooks';
import { useKeyboardShortcut } from '@/hooks/useKeyboardShortcut';
import { formatDuration, sortData } from '@/utils';
import PageHeader from '@/components/PageHeader';
import { RelativeTime } from '@/components/RelativeTime';
import SessionListStatsSidebar from '@/components/SessionListStatsSidebar';
import SessionsFilterDropdown from '@/components/SessionsFilterDropdown';
import SortableHeader from '@/components/SortableHeader';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import Quickstart from '@/components/Quickstart';
import SessionEmptyState from '@/components/SessionEmptyState';
import Chip from '@/components/Chip';
import { RepoIcon, BranchIcon, ComputerIcon, GitHubIcon, DurationIcon, PRIcon, CommitIcon, ClaudeCodeIcon } from '@/components/icons';
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
    selectedHostname,
    selectedPR,
    setSelectedPR,
    selectedCommit,
    setSelectedCommit,
    searchQuery,
    setSearchQuery,
    sortColumn,
    sortDirection,
    handleSort,
    handleRepoClick,
    handleHostnameClick,
    showEmptySessions,
    toggleShowEmptySessions,
  } = useSessionFilters();

  // Hidden keyboard shortcut to toggle showing empty sessions (for dev/debugging)
  const handleToggleEmptySessions = useCallback(() => {
    toggleShowEmptySessions();
  }, [toggleShowEmptySessions]);
  useKeyboardShortcut('mod+shift+e', handleToggleEmptySessions);
  const { sessions, loading, error } = useSessionsPolling(showSharedWithMe ? 'shared' : 'owned');
  const { message: successMessage, fading: successFading } = useSuccessMessage();

  // Get unique repos, branches, hostnames, PRs, and commits for filtering
  const { repos, branches, hostnames, prs, commits } = useMemo(() => {
    const repoSet = new Set<string>();
    const branchSet = new Set<string>();
    const hostnameSet = new Set<string>();
    const prSet = new Set<string>();
    const commitSet = new Set<string>();

    sessions.forEach((s) => {
      if (s.git_repo) repoSet.add(s.git_repo);
      if (s.git_branch) branchSet.add(s.git_branch);
      if (s.hostname) hostnameSet.add(s.hostname);
      s.github_prs?.forEach((pr) => prSet.add(pr));
      s.github_commits?.forEach((commit) => commitSet.add(commit));
    });

    return {
      repos: Array.from(repoSet).sort(),
      branches: Array.from(branchSet).sort(),
      hostnames: Array.from(hostnameSet).sort(),
      // Sort PRs numerically descending (newest first), with fallback for non-numeric values
      prs: Array.from(prSet).sort((a, b) => {
        const numA = Number(a);
        const numB = Number(b);
        if (Number.isNaN(numA) && Number.isNaN(numB)) return a.localeCompare(b);
        if (Number.isNaN(numA)) return 1; // Non-numeric sorts last
        if (Number.isNaN(numB)) return -1;
        return numB - numA;
      }),
      // Sort commits by most recent first (based on session last_sync_time)
      commits: Array.from(commitSet),
    };
  }, [sessions]);

  // Sorted and filtered sessions
  const sortedSessions = useMemo(() => {
    const lowerQuery = searchQuery.toLowerCase();
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
        // Filter by selected hostname
        if (selectedHostname && s.hostname !== selectedHostname) return false;
        // Filter by selected PR
        if (selectedPR && !s.github_prs?.includes(selectedPR)) return false;
        // Filter by selected commit
        if (selectedCommit && !s.github_commits?.includes(selectedCommit)) return false;
        // Filter by search query (match against title fields)
        if (lowerQuery) {
          const title = (s.custom_title || s.summary || s.first_user_message || '').toLowerCase();
          if (!title.includes(lowerQuery)) return false;
        }
        return true;
      },
    });
  }, [sessions, sortColumn, sortDirection, showSharedWithMe, selectedRepo, selectedBranch, selectedHostname, selectedPR, selectedCommit, searchQuery, showEmptySessions]);

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

  const hostnameCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (passesBaseFilters(s)) {
        const hostname = s.hostname || '';
        if (hostname) {
          counts[hostname] = (counts[hostname] || 0) + 1;
        }
      }
    });
    return counts;
  }, [sessions, passesBaseFilters]);

  const prCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (passesBaseFilters(s)) {
        // Only count PRs for sessions in the selected repo (if one is selected)
        if (!selectedRepo || s.git_repo === selectedRepo) {
          s.github_prs?.forEach((pr) => {
            counts[pr] = (counts[pr] || 0) + 1;
          });
        }
      }
    });
    return counts;
  }, [sessions, passesBaseFilters, selectedRepo]);

  const commitCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    sessions.forEach((s) => {
      if (passesBaseFilters(s)) {
        // Only count commits for sessions in the selected repo (if one is selected)
        if (!selectedRepo || s.git_repo === selectedRepo) {
          s.github_commits?.forEach((commit) => {
            counts[commit] = (counts[commit] || 0) + 1;
          });
        }
      }
    });
    return counts;
  }, [sessions, passesBaseFilters, selectedRepo]);

  const totalCount = useMemo(() => {
    return sessions.filter(passesBaseFilters).length;
  }, [sessions, passesBaseFilters]);

  const handleRowClick = (session: typeof sessions[0]) => {
    // CF-132: Use canonical URL for all session types
    navigate(`/sessions/${session.id}`);
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
              hostnames={showSharedWithMe ? [] : hostnames}
              prs={prs}
              commits={commits}
              selectedRepo={selectedRepo}
              selectedBranch={selectedBranch}
              selectedHostname={selectedHostname}
              selectedPR={selectedPR}
              selectedCommit={selectedCommit}
              repoCounts={repoCounts}
              branchCounts={branchCounts}
              hostnameCounts={hostnameCounts}
              prCounts={prCounts}
              commitCounts={commitCounts}
              totalCount={totalCount}
              searchQuery={searchQuery}
              onRepoClick={handleRepoClick}
              onBranchClick={(branch) => setSelectedBranch(branch)}
              onHostnameClick={handleHostnameClick}
              onPRClick={(pr) => setSelectedPR(pr)}
              onCommitClick={(commit) => setSelectedCommit(commit)}
              onSearchChange={setSearchQuery}
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
          {error && <Alert variant="error">{error.message}</Alert>}

          <div className={styles.card}>
            {loading && sessions.length === 0 ? (
              <p className={styles.loading}>Loading sessions...</p>
            ) : sortedSessions.length === 0 ? (
              showSharedWithMe ? (
                <SessionEmptyState variant="no-shared" />
              ) : selectedRepo || selectedBranch || selectedHostname || selectedPR || selectedCommit || searchQuery ? (
                <SessionEmptyState variant="no-matches" />
              ) : (
                <Quickstart />
              )
            ) : (
              <div className={styles.sessionsTable}>
                <table>
                  <thead>
                    <tr>
                      <SortableHeader
                        column="summary"
                        label="Session"
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
                        <td className={styles.sessionCell}>
                          <div className={session.custom_title || session.summary || session.first_user_message ? styles.sessionTitle : `${styles.sessionTitle} ${styles.untitled}`}>
                            {session.custom_title || session.summary || session.first_user_message || 'Untitled'}
                          </div>
                          <div className={styles.chipRow}>
                            <Chip icon={ClaudeCodeIcon} variant="neutral" copyValue={session.external_id}>
                              {session.external_id.substring(0, 8)}
                            </Chip>
                            {session.git_repo && (
                              <Chip
                                icon={session.git_repo_url?.includes('github.com') ? GitHubIcon : RepoIcon}
                                variant="neutral"
                                copyValue={session.git_repo_url || session.git_repo}
                              >
                                {session.git_repo}
                              </Chip>
                            )}
                            {session.git_branch && (
                              <Chip
                                icon={BranchIcon}
                                variant="blue"
                                copyValue={session.git_repo_url ? `${session.git_repo_url}/tree/${session.git_branch}` : session.git_branch}
                              >
                                {session.git_branch}
                              </Chip>
                            )}
                            {session.github_prs?.map((pr) => (
                              <Chip
                                key={pr}
                                icon={PRIcon}
                                variant="purple"
                                copyValue={session.git_repo_url ? `${session.git_repo_url}/pull/${pr}` : pr}
                              >
                                #{pr}
                              </Chip>
                            ))}
                            {session.github_commits?.[0] && (
                              <Chip
                                icon={CommitIcon}
                                variant="purple"
                                copyValue={session.git_repo_url ? `${session.git_repo_url}/commit/${session.github_commits[0]}` : session.github_commits[0]}
                              >
                                {session.github_commits[0].slice(0, 7)}
                              </Chip>
                            )}
                            {!showSharedWithMe && session.hostname && (
                              <Chip icon={ComputerIcon} variant="green" copyValue={session.hostname}>
                                {session.hostname}
                              </Chip>
                            )}
                          </div>
                        </td>
                        <td className={styles.timestamp}>
                          <span className={styles.activityContent}>
                            <span className={styles.activityTime}>
                              {session.last_sync_time ? <RelativeTime date={session.last_sync_time} /> : '-'}
                            </span>
                            {session.first_seen && session.last_sync_time && (
                              <span className={styles.activityDuration}>
                                {DurationIcon}
                                {formatDuration(new Date(session.last_sync_time).getTime() - new Date(session.first_seen).getTime())}
                              </span>
                            )}
                          </span>
                        </td>
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
