import { useState, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useSessions } from '@/hooks/useSessions';
import { formatDate } from '@/utils/utils';
import { sortData, type SortDirection } from '@/utils/sorting';
import SortableHeader from '@/components/SortableHeader';
import Alert from '@/components/Alert';
import styles from './SessionsPage.module.css';

type SortColumn = 'title' | 'session_id' | 'last_run_time';

function SessionsPage() {
  const navigate = useNavigate();
  const { sessions, loading, error } = useSessions();
  const [sortColumn, setSortColumn] = useState<SortColumn>('last_run_time');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');

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

  // Sorted sessions - filter out empty sessions (0 byte transcripts)
  const sortedSessions = useMemo(() => {
    return sortData({
      data: sessions,
      sortBy: sortColumn,
      direction: sortDirection,
      filter: (s) => s.max_transcript_size > 0,
    });
  }, [sessions, sortColumn, sortDirection]);

  const handleRowClick = (sessionId: string) => {
    navigate(`/sessions/${sessionId}`);
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>Sessions</h1>
        <Link to="/" className={styles.btnLink}>
          ← Back to Home
        </Link>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      <div className={styles.card}>
        {loading ? (
          <p className={styles.loading}>Loading sessions...</p>
        ) : sessions.length === 0 ? (
          <p className={styles.empty}>
            No sessions yet. Sessions will appear here after you use confab.
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
                  <th>Git Repo</th>
                  <th>Git Branch</th>
                  <SortableHeader
                    column="session_id"
                    label="Session ID"
                    currentColumn={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableHeader
                    column="last_run_time"
                    label="Last Activity"
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
                    onClick={() => handleRowClick(session.session_id)}
                  >
                    <td className={session.title ? '' : styles.sessionTitle}>
                      {session.title || 'Untitled Session'}
                    </td>
                    <td className={styles.gitInfo}>{session.git_repo || '—'}</td>
                    <td className={styles.gitInfo}>{session.git_branch || '—'}</td>
                    <td>
                      <code className={styles.sessionId}>{session.session_id.substring(0, 8)}</code>
                    </td>
                    <td>{formatDate(session.last_run_time)}</td>
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
