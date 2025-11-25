import { useState, useMemo, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useSessions } from '@/hooks/useSessions';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import { formatRelativeTime } from '@/utils/utils';
import { sortData, type SortDirection } from '@/utils/sorting';
import SortableHeader from '@/components/SortableHeader';
import Alert from '@/components/Alert';
import styles from './SessionsPage.module.css';

type SortColumn = 'title' | 'session_id' | 'last_run_time';

function SessionsPage() {
  useDocumentTitle('Sessions');
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [showSharedWithMe, setShowSharedWithMe] = useState(false);
  const { sessions, loading, error } = useSessions(showSharedWithMe);
  const [sortColumn, setSortColumn] = useState<SortColumn>('last_run_time');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const [successMessage, setSuccessMessage] = useState('');
  const [successFading, setSuccessFading] = useState(false);

  useEffect(() => {
    // Check for success message from URL params
    const successParam = searchParams.get('success');
    if (successParam) {
      setSuccessMessage(successParam);
      setSuccessFading(false);
      // Remove the success param from URL
      searchParams.delete('success');
      setSearchParams(searchParams, { replace: true });
      // Start fade out after 4.5 seconds, then remove after animation completes
      setTimeout(() => setSuccessFading(true), 4500);
      setTimeout(() => setSuccessMessage(''), 5000);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      // Toggle direction if clicking the same column
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      // New column: default to ascending (except last_run_time defaults to descending)
      setSortColumn(column);
      setSortDirection(column === 'last_run_time' ? 'desc' : 'asc');
    }
  };

  // Sorted sessions - filter based on ownership and empty sessions
  const sortedSessions = useMemo(() => {
    return sortData({
      data: sessions,
      sortBy: sortColumn,
      direction: sortDirection,
      filter: (s) => {
        // Filter out empty sessions
        if (s.max_transcript_size <= 0) return false;
        // Show only owned or only shared based on toggle
        return showSharedWithMe ? !s.is_owner : s.is_owner;
      },
    });
  }, [sessions, sortColumn, sortDirection, showSharedWithMe]);

  const handleRowClick = (session: typeof sessions[0]) => {
    // For shared sessions, use the share URL. For owned sessions, use normal URL
    if (session.is_owner) {
      navigate(`/sessions/${session.session_id}`);
    } else {
      // Shared session - use share token URL
      navigate(`/sessions/${session.session_id}/shared/${session.share_token}`);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>{showSharedWithMe ? 'Shared with me' : 'Sessions'}</h1>
        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${!showSharedWithMe ? styles.active : ''}`}
            onClick={() => setShowSharedWithMe(false)}
          >
            My Sessions
          </button>
          <button
            className={`${styles.tab} ${showSharedWithMe ? styles.active : ''}`}
            onClick={() => setShowSharedWithMe(true)}
          >
            Shared with me
          </button>
        </div>
      </div>

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
                    column="session_id"
                    label="ID"
                    currentColumn={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableHeader
                    column="last_run_time"
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
                    key={session.session_id}
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
                    <td className={styles.sessionId}>{session.session_id.substring(0, 8)}</td>
                    <td className={styles.timestamp}>{formatRelativeTime(session.last_run_time)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

export default SessionsPage;
