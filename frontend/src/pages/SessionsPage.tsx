import { useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessionsFetch, useAuth, useDocumentTitle, useSuccessMessage, useSessionFilters } from '@/hooks';
import { formatDuration } from '@/utils';
import { RelativeTime } from '@/components/RelativeTime';
import FilterChipsBar from '@/components/FilterChipsBar';
import Pagination from '@/components/Pagination';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import Alert from '@/components/Alert';
import Quickstart from '@/components/Quickstart';
import SessionEmptyState from '@/components/SessionEmptyState';
import Chip from '@/components/Chip';
import { RepoIcon, BranchIcon, GitHubIcon, DurationIcon, PRIcon, CommitIcon, ClaudeCodeIcon, RefreshIcon } from '@/components/icons';
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
    repos, branches, owners, query,
    toggleRepo, toggleBranch, toggleOwner,
    setQuery, clearAll,
  } = useSessionFilters();

  const { sessions, hasMore, filterOptions, loading, error, refetch, goNext, goPrev, canGoPrev } = useSessionsFetch({
    repos, branches, owners, query,
  });
  const { user } = useAuth();
  const { message: successMessage, fading: successFading } = useSuccessMessage();

  const hasActiveFilters = repos.length > 0 || branches.length > 0 || owners.length > 0 || query !== '';

  const handleRowClick = (sessionId: string) => {
    navigate(`/sessions/${sessionId}`);
  };

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <header className={styles.toolbar}>
          <div className={styles.toolbarTop}>
            <span className={styles.sessionCount}>
              {loading && sessions.length > 0 ? (
                <span className={styles.loadingIndicator}>Updating...</span>
              ) : (
                'Sessions'
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
                title="Refresh sessions"
                aria-label="Refresh sessions"
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
            {loading && sessions.length === 0 && (
              <p className={styles.loading}>Loading sessions...</p>
            )}
            {!loading && sessions.length === 0 && (
              hasActiveFilters ? <SessionEmptyState /> : <Quickstart />
            )}
            {sessions.length > 0 && (
              <div className={`${styles.sessionsTable} ${loading ? styles.tableLoading : ''}`}>
                <table>
                  <thead>
                    <tr>
                      <th>Session</th>
                      <th>Activity</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sessions.map((session) => {
                      const title = getSessionTitle(session);
                      return (
                        <tr
                          key={session.id}
                          className={`${styles.clickableRow} ${!session.is_owner ? styles.sharedRow : ''}`}
                          onClick={() => handleRowClick(session.id)}
                        >
                          <td className={styles.sessionCell}>
                            <div className={title ? styles.sessionTitle : `${styles.sessionTitle} ${styles.untitled}`}>
                              {title || 'Untitled'}
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
                      );
                    })}
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
