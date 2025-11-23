import { useState, useEffect, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import type { Session } from '@/types';
import { formatDate } from '@/utils/utils';
import styles from './SessionsPage.module.css';

type SortColumn = 'title' | 'session_id' | 'last_run_time';
type SortDirection = 'asc' | 'desc';

function SessionsPage() {
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [sortColumn, setSortColumn] = useState<SortColumn>('last_run_time');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');

  useEffect(() => {
    fetchSessions();
  }, []);

  const fetchSessions = async () => {
    setLoading(true);
    setError('');
    try {
      const response = await fetch('/api/v1/sessions', {
        credentials: 'include',
      });

      if (response.status === 401) {
        window.location.href = '/';
        return;
      }

      if (!response.ok) {
        throw new Error('Failed to fetch sessions');
      }

      const data = await response.json();
      setSessions(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load sessions');
    } finally {
      setLoading(false);
    }
  };

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
    // Filter out sessions where all runs have 0-byte transcripts
    const filtered = sessions.filter((s) => s.max_transcript_size > 0);

    const sorted = [...filtered];
    sorted.sort((a, b) => {
      let aVal: string | number;
      let bVal: string | number;

      switch (sortColumn) {
        case 'title':
          aVal = a.title || 'Untitled Session';
          bVal = b.title || 'Untitled Session';
          break;
        case 'session_id':
          aVal = a.session_id;
          bVal = b.session_id;
          break;
        case 'last_run_time':
          aVal = new Date(a.last_run_time).getTime();
          bVal = new Date(b.last_run_time).getTime();
          break;
      }

      if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1;
      if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1;
      return 0;
    });
    return sorted;
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

      {error && <div className={styles.alertError}>{error}</div>}

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
                  <th className={styles.sortable} onClick={() => handleSort('title')}>
                    Title
                    {sortColumn === 'title' && (
                      <span className={styles.sortIndicator}>
                        {sortDirection === 'asc' ? '↑' : '↓'}
                      </span>
                    )}
                  </th>
                  <th>Git Repo</th>
                  <th>Git Branch</th>
                  <th className={styles.sortable} onClick={() => handleSort('session_id')}>
                    Session ID
                    {sortColumn === 'session_id' && (
                      <span className={styles.sortIndicator}>
                        {sortDirection === 'asc' ? '↑' : '↓'}
                      </span>
                    )}
                  </th>
                  <th className={styles.sortable} onClick={() => handleSort('last_run_time')}>
                    Last Activity
                    {sortColumn === 'last_run_time' && (
                      <span className={styles.sortIndicator}>
                        {sortDirection === 'asc' ? '↑' : '↓'}
                      </span>
                    )}
                  </th>
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
