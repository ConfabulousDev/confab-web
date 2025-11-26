import { useState, useEffect, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { sharesAPI, sessionsAPI, AuthenticationError } from '@/services/api';
import { useDocumentTitle, useCopyToClipboard, useSuccessMessage } from '@/hooks';
import type { SessionShare } from '@/types';
import { formatRelativeTime, sortData, type SortDirection } from '@/utils';
import SortableHeader from '@/components/SortableHeader';
import Alert from '@/components/Alert';
import styles from './ShareLinksPage.module.css';

type SortColumn = 'session_title' | 'visibility' | 'created_at' | 'expires_at';

function ShareLinksPage() {
  useDocumentTitle('Share Links');
  const navigate = useNavigate();
  const [shares, setShares] = useState<SessionShare[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const { message: successMessage, setMessage: setSuccessMessage } = useSuccessMessage({
    skipUrlParams: true,
  });
  const { copy, message: copyMessage } = useCopyToClipboard({
    successMessage: 'Link copied to clipboard!',
  });
  const [sortColumn, setSortColumn] = useState<SortColumn>('created_at');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      // Default to descending for dates, ascending for others
      setSortDirection(column === 'created_at' || column === 'expires_at' ? 'desc' : 'asc');
    }
  };

  const sortedShares = useMemo(() => {
    return sortData({
      data: shares,
      sortBy: sortColumn,
      direction: sortDirection,
    });
  }, [shares, sortColumn, sortDirection]);

  useEffect(() => {
    fetchShares();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fetchShares is intentionally omitted; we only want to fetch on mount
  }, []);

  async function fetchShares() {
    setLoading(true);
    setError('');
    try {
      const data = await sharesAPI.list();
      setShares(data);
    } catch (err) {
      if (err instanceof AuthenticationError) {
        navigate('/');
        return;
      }
      setError(err instanceof Error ? err.message : 'Failed to load shares');
    } finally {
      setLoading(false);
    }
  }

  async function handleRevoke(shareToken: string) {
    if (!confirm('Are you sure you want to revoke this share?')) {
      return;
    }

    setError('');
    try {
      await sessionsAPI.revokeShare(shareToken);
      setSuccessMessage('Share link revoked successfully');
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke share');
    }
  }

  function getShareURL(sessionId: string, shareToken: string): string {
    return `${window.location.origin}/sessions/${sessionId}/shared/${shareToken}`;
  }

  // Display either copy message or other success messages
  const displayMessage = copyMessage || successMessage;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>Share Links</h1>
      </div>

      {displayMessage && <Alert variant="success">✓ {displayMessage}</Alert>}
      {error && <Alert variant="error">{error}</Alert>}

      <div className={styles.card}>
        {loading ? (
          <p className={styles.loading}>Loading shares...</p>
        ) : shares.length === 0 ? (
          <p className={styles.empty}>
            No share links yet. Share a session to see links here.
          </p>
        ) : (
          <div className={styles.sharesTable}>
            <table>
              <thead>
                <tr>
                  <SortableHeader
                    column="session_title"
                    label="Session"
                    currentColumn={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableHeader
                    column="visibility"
                    label="Visibility"
                    currentColumn={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <th>Invited Emails</th>
                  <SortableHeader
                    column="created_at"
                    label="Created"
                    currentColumn={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableHeader
                    column="expires_at"
                    label="Expires"
                    currentColumn={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sortedShares.map((share) => {
                  const shareURL = getShareURL(share.session_id, share.share_token);
                  const isExpired = share.expires_at && new Date(share.expires_at) < new Date();

                  return (
                    <tr key={share.share_token} className={isExpired ? styles.expiredRow : ''}>
                      <td>
                        <Link to={`/sessions/${share.session_id}`} className={styles.sessionLink}>
                          {share.session_title || 'Untitled Session'}
                        </Link>
                        <div className={styles.sessionId}>
                          <code>{share.external_id.substring(0, 8)}</code>
                        </div>
                      </td>
                      <td>
                        <span className={`${styles.badge} ${styles[share.visibility]}`}>
                          {share.visibility}
                        </span>
                      </td>
                      <td className={styles.emails}>
                        {share.visibility === 'private' && share.invited_emails && share.invited_emails.length > 0
                          ? share.invited_emails.join(', ')
                          : '—'}
                      </td>
                      <td className={styles.timestamp}>{formatRelativeTime(share.created_at)}</td>
                      <td>
                        {share.expires_at ? (
                          <span className={isExpired ? styles.expired : styles.timestamp}>
                            {formatRelativeTime(share.expires_at)}
                            {isExpired && ' (Expired)'}
                          </span>
                        ) : (
                          <span className={styles.neverExpires}>Never</span>
                        )}
                      </td>
                      <td>
                        <div className={styles.actions}>
                          <button
                            className={`${styles.btn} ${styles.btnCopy}`}
                            onClick={() => copy(shareURL)}
                            title="Copy link"
                          >
                            Copy Link
                          </button>
                          <button
                            className={`${styles.btn} ${styles.btnDanger}`}
                            onClick={() => handleRevoke(share.share_token)}
                          >
                            Revoke
                          </button>
                        </div>
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
  );
}

export default ShareLinksPage;
