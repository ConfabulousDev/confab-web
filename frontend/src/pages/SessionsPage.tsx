import { useCallback, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessionsFetch, useAuth, useDocumentTitle, useSuccessMessage, useSessionFilters } from '@/hooks';
import { useKeyboardShortcut } from '@/hooks/useKeyboardShortcut';
import { formatDuration, sortData } from '@/utils';
import PageHeader from '@/components/PageHeader';
import { RelativeTime } from '@/components/RelativeTime';
import SessionsFilterDropdown from '@/components/SessionsFilterDropdown';
import SortableHeader from '@/components/SortableHeader';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import Quickstart from '@/components/Quickstart';
import SessionEmptyState from '@/components/SessionEmptyState';
import Chip from '@/components/Chip';
import { RepoIcon, BranchIcon, ComputerIcon, GitHubIcon, DurationIcon, PRIcon, CommitIcon, ClaudeCodeIcon, RefreshIcon } from '@/components/icons';
import styles from './SessionsPage.module.css';

// Strip .git suffix from repo URLs for clean GitHub links
function cleanRepoUrl(url: string): string {
  return url.replace(/\.git$/, '');
}

// Derive display title from session fields with fallback chain
function getSessionTitle(session: { custom_title?: string | null; suggested_session_title?: string | null; summary?: string | null; first_user_message?: string | null }): string | null {
  return session.custom_title || session.suggested_session_title || session.summary || session.first_user_message || null;
}

function SessionsPage() {
  useDocumentTitle('Sessions');
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const {
    selectedRepo,
    selectedBranch,
    setSelectedBranch,
    selectedHostname,
    selectedOwner,
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
    handleOwnerClick,
    showEmptySessions,
    toggleShowEmptySessions,
  } = useSessionFilters();

  // Hidden keyboard shortcut to toggle showing empty sessions (for dev/debugging)
  useKeyboardShortcut('mod+shift+e', toggleShowEmptySessions);
  const { sessions, loading, error, refetch } = useSessionsFetch();
  const { user } = useAuth();
  const { message: successMessage, fading: successFading } = useSuccessMessage();

  // Helper to derive owner email for a session
  const getOwnerEmail = useCallback((s: typeof sessions[0]) => {
    return s.is_owner ? (user?.email ?? '') : (s.shared_by_email ?? '');
  }, [user?.email]);

  // Helper to check if a session passes the base filters (excluding repo/branch)
  const passesBaseFilters = useCallback((s: typeof sessions[0]) => {
    if (s.total_lines <= 0) return false;
    if (!showEmptySessions && !s.summary && !s.first_user_message) return false;
    return true;
  }, [showEmptySessions]);

  // Get unique repos, branches, hostnames, owners, and PRs for filtering
  const { repos, branches, hostnames, owners, prs } = useMemo(() => {
    const repoSet = new Set<string>();
    const branchSet = new Set<string>();
    const hostnameSet = new Set<string>();
    const ownerMap = new Map<string, string>();
    const prSet = new Set<string>();
    const currentUserEmail = user?.email?.toLowerCase() ?? '';

    sessions.forEach((s) => {
      if (!passesBaseFilters(s)) return;
      if (s.git_repo) repoSet.add(s.git_repo);
      if (s.git_branch) branchSet.add(s.git_branch);
      if (s.hostname) hostnameSet.add(s.hostname);
      const ownerEmail = getOwnerEmail(s);
      if (ownerEmail) {
        const key = ownerEmail.toLowerCase();
        if (!ownerMap.has(key)) ownerMap.set(key, ownerEmail);
      }
      s.github_prs?.forEach((pr) => prSet.add(pr));
    });

    // Sort owners: current user first, then alphabetically
    const sortedOwners = Array.from(ownerMap.entries())
      .sort(([a], [b]) => {
        if (a === currentUserEmail) return -1;
        if (b === currentUserEmail) return 1;
        return a.localeCompare(b);
      })
      .map(([, display]) => display);

    return {
      repos: Array.from(repoSet).sort(),
      branches: Array.from(branchSet).sort(),
      hostnames: Array.from(hostnameSet).sort(),
      owners: sortedOwners,
      // Sort PRs numerically descending (newest first), with fallback for non-numeric values
      prs: Array.from(prSet).sort((a, b) => {
        const numA = Number(a);
        const numB = Number(b);
        if (Number.isNaN(numA) && Number.isNaN(numB)) return a.localeCompare(b);
        if (Number.isNaN(numA)) return 1; // Non-numeric sorts last
        if (Number.isNaN(numB)) return -1;
        return numB - numA;
      }),
    };
  }, [sessions, passesBaseFilters, getOwnerEmail, user?.email]);

  // Sorted and filtered sessions
  const sortedSessions = useMemo(() => {
    const lowerQuery = searchQuery.toLowerCase();
    return sortData({
      data: sessions,
      sortBy: sortColumn,
      direction: sortDirection,
      filter: (s) => {
        if (!passesBaseFilters(s)) return false;
        // Filter by selected repo
        if (selectedRepo && s.git_repo !== selectedRepo) return false;
        // Filter by selected branch
        if (selectedBranch && s.git_branch !== selectedBranch) return false;
        // Filter by selected hostname
        if (selectedHostname && s.hostname !== selectedHostname) return false;
        // Filter by selected owner
        if (selectedOwner && getOwnerEmail(s).toLowerCase() !== selectedOwner.toLowerCase()) return false;
        // Filter by selected PR
        if (selectedPR && !s.github_prs?.includes(selectedPR)) return false;
        // Filter by commit search (prefix match)
        if (selectedCommit) {
          const commitLower = selectedCommit.toLowerCase();
          const hasMatch = s.github_commits?.some((c) => c.toLowerCase().startsWith(commitLower));
          if (!hasMatch) return false;
        }
        // Filter by search query (match against title fields)
        if (lowerQuery) {
          const title = (getSessionTitle(s) || '').toLowerCase();
          if (!title.includes(lowerQuery)) return false;
        }
        return true;
      },
    });
  }, [sessions, sortColumn, sortDirection, selectedRepo, selectedBranch, selectedHostname, selectedOwner, selectedPR, selectedCommit, searchQuery, passesBaseFilters, getOwnerEmail]);

  // Count sessions per filter dimension in a single pass
  const { repoCounts, branchCounts, hostnameCounts, ownerCounts, prCounts, totalCount } = useMemo(() => {
    const repo: Record<string, number> = {};
    const branch: Record<string, number> = {};
    const hostname: Record<string, number> = {};
    const owner: Record<string, number> = {};
    const pr: Record<string, number> = {};
    let total = 0;

    for (const s of sessions) {
      if (!passesBaseFilters(s)) continue;
      total++;

      const repoName = s.git_repo || '';
      repo[repoName] = (repo[repoName] || 0) + 1;

      if (s.hostname) {
        hostname[s.hostname] = (hostname[s.hostname] || 0) + 1;
      }

      const ownerEmail = getOwnerEmail(s);
      if (ownerEmail) {
        const key = ownerEmail.toLowerCase();
        owner[key] = (owner[key] || 0) + 1;
      }

      // Branch and PR counts are scoped to the selected repo
      if (!selectedRepo || s.git_repo === selectedRepo) {
        const branchName = s.git_branch || '';
        branch[branchName] = (branch[branchName] || 0) + 1;

        s.github_prs?.forEach((p) => {
          pr[p] = (pr[p] || 0) + 1;
        });
      }
    }

    return {
      repoCounts: repo,
      branchCounts: branch,
      hostnameCounts: hostname,
      ownerCounts: owner,
      prCounts: pr,
      totalCount: total,
    };
  }, [sessions, passesBaseFilters, getOwnerEmail, selectedRepo]);

  const handleRowClick = (session: typeof sessions[0]) => {
    // CF-132: Use canonical URL for all session types
    navigate(`/sessions/${session.id}`);
  };

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader
          leftContent={
            <span className={styles.sessionCount}>
              {sortedSessions.length} session{sortedSessions.length !== 1 ? 's' : ''}
              {showEmptySessions && <span className={styles.devIndicator}> (showing empty)</span>}
            </span>
          }
          actions={
            <div className={styles.headerActions}>
              <button
                className={styles.refreshBtn}
                onClick={() => refetch()}
                title="Refresh sessions"
                aria-label="Refresh sessions"
                disabled={loading}
              >
                {RefreshIcon}
              </button>
              <SessionsFilterDropdown
                repos={repos}
                branches={branches}
                hostnames={hostnames}
                owners={owners}
                prs={prs}
                selectedRepo={selectedRepo}
                selectedBranch={selectedBranch}
                selectedHostname={selectedHostname}
                selectedOwner={selectedOwner}
                selectedPR={selectedPR}
                commitSearch={selectedCommit || ''}
                repoCounts={repoCounts}
                branchCounts={branchCounts}
                hostnameCounts={hostnameCounts}
                ownerCounts={ownerCounts}
                prCounts={prCounts}
                totalCount={totalCount}
                searchQuery={searchQuery}
                currentUserEmail={user?.email ?? null}
                onRepoClick={handleRepoClick}
                onBranchClick={(branch) => setSelectedBranch(branch)}
                onHostnameClick={handleHostnameClick}
                onOwnerClick={handleOwnerClick}
                onPRClick={(pr) => setSelectedPR(pr)}
                onCommitSearchChange={(commit) => setSelectedCommit(commit || null)}
                onSearchChange={setSearchQuery}
              />
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
          {error && <Alert variant="error">{error.message}</Alert>}

          <div className={styles.card}>
            {loading && sessions.length === 0 ? (
              <p className={styles.loading}>Loading sessions...</p>
            ) : sortedSessions.length === 0 ? (
              selectedRepo || selectedBranch || selectedHostname || selectedOwner || selectedPR || selectedCommit || searchQuery ? (
                <SessionEmptyState />
              ) : sessions.some(s => s.is_owner) ? (
                <SessionEmptyState />
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
                        className={`${styles.clickableRow} ${!session.is_owner ? styles.sharedRow : ''}`}
                        onClick={() => handleRowClick(session)}
                      >
                        <td className={styles.sessionCell}>
                          <div className={getSessionTitle(session) ? styles.sessionTitle : `${styles.sessionTitle} ${styles.untitled}`}>
                            {getSessionTitle(session) || 'Untitled'}
                          </div>
                          <div className={styles.chipRow}>
                            <Chip icon={ClaudeCodeIcon} variant="neutral" copyValue={session.external_id}>
                              {session.external_id.substring(0, 8)}
                            </Chip>
                            {session.git_repo && (
                              <Chip
                                icon={session.git_repo_url?.includes('github.com') ? GitHubIcon : RepoIcon}
                                variant="neutral"
                                copyValue={session.git_repo_url ? cleanRepoUrl(session.git_repo_url) : session.git_repo}
                              >
                                {session.git_repo}
                              </Chip>
                            )}
                            {session.git_branch && (
                              <Chip
                                icon={BranchIcon}
                                variant="blue"
                                copyValue={session.git_repo_url ? `${cleanRepoUrl(session.git_repo_url)}/tree/${session.git_branch}` : session.git_branch}
                              >
                                {session.git_branch}
                              </Chip>
                            )}
                            {session.github_prs?.map((pr) => (
                              <Chip
                                key={pr}
                                icon={PRIcon}
                                variant="purple"
                                copyValue={session.git_repo_url ? `${cleanRepoUrl(session.git_repo_url)}/pull/${pr}` : pr}
                              >
                                #{pr}
                              </Chip>
                            ))}
                            {session.github_commits?.[0] && (
                              <Chip
                                icon={CommitIcon}
                                variant="purple"
                                copyValue={session.git_repo_url ? `${cleanRepoUrl(session.git_repo_url)}/commit/${session.github_commits[0]}` : session.github_commits[0]}
                              >
                                {session.github_commits[0].slice(0, 7)}
                              </Chip>
                            )}
                            {session.hostname && (
                              <Chip icon={ComputerIcon} variant="green" copyValue={session.hostname}>
                                {session.hostname}
                              </Chip>
                            )}
                          </div>
                          {session.shared_by_email && (
                            <div className={styles.sharedByLine}>
                              Shared by {session.shared_by_email}
                            </div>
                          )}
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
