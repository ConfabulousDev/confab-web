import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSessions, useDocumentTitle, useSuccessMessage } from '@/hooks';
import { formatRelativeTime, sortData, type SortDirection } from '@/utils';
import SortableHeader from '@/components/SortableHeader';
import Alert from '@/components/Alert';
import styles from './SessionsPage.module.css';

type SortColumn = 'title' | 'external_id' | 'last_run_time';

function SessionsPage() {
  useDocumentTitle('Sessions');
  const navigate = useNavigate();
  const [showSharedWithMe, setShowSharedWithMe] = useState(false);
  const { sessions, loading, error } = useSessions(showSharedWithMe);
  const [sortColumn, setSortColumn] = useState<SortColumn>('last_run_time');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const { message: successMessage, fading: successFading } = useSuccessMessage();

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
      navigate(`/sessions/${session.id}`);
    } else {
      // Shared session - use share token URL
      navigate(`/sessions/${session.id}/shared/${session.share_token}`);
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
                    column="external_id"
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
