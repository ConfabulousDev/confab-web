import { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { fetchWithCSRF } from '@/services/csrf';
import { formatDate } from '@/utils/utils';
import Alert from '@/components/Alert';
import styles from './ShareLinksPage.module.css';

interface ShareLink {
  id: number;
  session_id: string;
  session_title?: string;
  share_token: string;
  visibility: 'public' | 'private';
  invited_emails?: string[];
  expires_at?: string;
  created_at: string;
  last_accessed_at?: string;
}

function ShareLinksPage() {
  const navigate = useNavigate();
  const [shares, setShares] = useState<ShareLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [successMessage, setSuccessMessage] = useState('');

  useEffect(() => {
    fetchShares();
  }, []);

  async function fetchShares() {
    setLoading(true);
    setError('');
    try {
      const response = await fetch('/api/v1/shares', {
        credentials: 'include',
      });

      if (response.status === 401) {
        navigate('/');
        return;
      }

      if (!response.ok) {
        throw new Error('Failed to fetch shares');
      }

      const data: ShareLink[] = await response.json();
      setShares(data);
    } catch (err) {
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
      const response = await fetchWithCSRF(`/api/v1/shares/${shareToken}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to revoke share');
      }

      setSuccessMessage('Share link revoked successfully');
      setTimeout(() => setSuccessMessage(''), 5000);
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke share');
    }
  }

  function getShareURL(sessionId: string, shareToken: string): string {
    return `${window.location.origin}/sessions/${sessionId}/shared/${shareToken}`;
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
    setSuccessMessage('Link copied to clipboard!');
    setTimeout(() => setSuccessMessage(''), 3000);
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>Share Links</h1>
        <Link to="/" className={styles.btnLink}>
          ← Back to Home
        </Link>
      </div>

      {successMessage && <Alert variant="success">✓ {successMessage}</Alert>}
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
                  <th>Session</th>
                  <th>Visibility</th>
                  <th>Invited Emails</th>
                  <th>Expires</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {shares.map((share) => {
                  const shareURL = getShareURL(share.session_id, share.share_token);
                  const isExpired = share.expires_at && new Date(share.expires_at) < new Date();

                  return (
                    <tr key={share.share_token} className={isExpired ? styles.expiredRow : ''}>
                      <td>
                        <Link to={`/sessions/${share.session_id}`} className={styles.sessionLink}>
                          {share.session_title || 'Untitled Session'}
                        </Link>
                        <div className={styles.sessionId}>
                          <code>{share.session_id.substring(0, 8)}</code>
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
                      <td>
                        {share.expires_at ? (
                          <span className={isExpired ? styles.expired : ''}>
                            {formatDate(share.expires_at)}
                            {isExpired && ' (Expired)'}
                          </span>
                        ) : (
                          <span className={styles.neverExpires}>Never</span>
                        )}
                      </td>
                      <td>{formatDate(share.created_at)}</td>
                      <td>
                        <div className={styles.actions}>
                          <button
                            className={`${styles.btn} ${styles.btnCopy}`}
                            onClick={() => copyToClipboard(shareURL)}
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
