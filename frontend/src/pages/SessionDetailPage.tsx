import { useState, useEffect } from 'react';
import { useParams, useSearchParams, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { fetchWithCSRF } from '@/services/csrf';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import type { SessionDetail, SessionShare, RunDetail } from '@/types';
import { formatRelativeTime, formatDate } from '@/utils/utils';
import RunCard from '@/components/RunCard';
import styles from './SessionDetailPage.module.css';

function SessionDetailPage() {
  const { id: sessionId } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [successMessage, setSuccessMessage] = useState('');
  const [successFading, setSuccessFading] = useState(false);

  // Share dialog state
  const [showShareDialog, setShowShareDialog] = useState(false);
  const [shareVisibility, setShareVisibility] = useState<'public' | 'private'>('public');
  const [invitedEmails, setInvitedEmails] = useState<string[]>([]);
  const [newEmail, setNewEmail] = useState('');
  const [expiresInDays, setExpiresInDays] = useState<number | null>(7);
  const [createdShareURL, setCreatedShareURL] = useState('');
  const [shares, setShares] = useState<SessionShare[]>([]);
  const [loadingShares, setLoadingShares] = useState(false);

  // Delete dialog state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleteMode, setDeleteMode] = useState<'session' | 'version'>('session');
  const [selectedDeleteRunIndex, setSelectedDeleteRunIndex] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Run selection state
  const [selectedRunIndex, setSelectedRunIndex] = useState(0);

  const selectedRun: RunDetail | undefined = session?.runs[selectedRunIndex];

  // Dynamic page title based on session
  const pageTitle = session ? `Session ${session.session_id.substring(0, 8)}` : 'Session';
  useDocumentTitle(pageTitle);

  useEffect(() => {
    if (sessionId) {
      fetchSession();
    }

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
  }, [sessionId]);

  async function fetchSession() {
    if (!sessionId) return;

    setLoading(true);
    setError('');
    try {
      const response = await fetch(`/api/v1/sessions/${sessionId}`, {
        credentials: 'include',
      });

      if (response.status === 401) {
        window.location.href = '/';
        return;
      }

      if (response.status === 404) {
        setError('Session not found');
        setLoading(false);
        return;
      }

      if (!response.ok) {
        throw new Error('Failed to fetch session');
      }

      const data: SessionDetail = await response.json();
      setSession(data);

      // Set initial selection to the latest run by timestamp
      if (data.runs && data.runs.length > 0) {
        const latestIndex = data.runs.reduce((latestIdx, run, idx) => {
          return new Date(run.end_timestamp) > new Date(data.runs[latestIdx].end_timestamp) ? idx : latestIdx;
        }, 0);
        setSelectedRunIndex(latestIndex);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load session');
    } finally {
      setLoading(false);
    }
  }

  async function fetchShares() {
    if (!sessionId) return;

    setLoadingShares(true);
    try {
      const response = await fetch(`/api/v1/sessions/${sessionId}/shares`, {
        credentials: 'include',
      });
      if (response.ok) {
        const data = await response.json();
        setShares(data);
      }
    } catch (err) {
      console.error('Failed to load shares:', err);
    } finally {
      setLoadingShares(false);
    }
  }

  function openShareDialog() {
    setShowShareDialog(true);
    setCreatedShareURL('');
    setShareVisibility('public');
    setInvitedEmails([]);
    setNewEmail('');
    setExpiresInDays(7);
    fetchShares();
  }

  function addEmail() {
    const email = newEmail.trim();
    if (email && email.includes('@') && !invitedEmails.includes(email)) {
      setInvitedEmails([...invitedEmails, email]);
      setNewEmail('');
    }
  }

  function removeEmail(email: string) {
    setInvitedEmails(invitedEmails.filter((e) => e !== email));
  }

  async function createShare() {
    if (!sessionId) return;

    setError('');
    try {
      const response = await fetchWithCSRF(`/api/v1/sessions/${sessionId}/share`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          visibility: shareVisibility,
          invited_emails: shareVisibility === 'private' ? invitedEmails : [],
          expires_in_days: expiresInDays,
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to create share');
      }

      const result = await response.json();
      setCreatedShareURL(result.share_url);
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create share');
    }
  }

  async function revokeShare(shareToken: string) {
    if (!confirm('Are you sure you want to revoke this share?')) {
      return;
    }

    try {
      const response = await fetchWithCSRF(`/api/v1/shares/${shareToken}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to revoke share');
      }

      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke share');
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
    alert('Copied to clipboard!');
  }

  function openDeleteDialog() {
    setShowDeleteDialog(true);
    setDeleteMode('session');
    setSelectedDeleteRunIndex(null);
    setError('');
  }

  async function handleDelete() {
    if (!sessionId || !session) return;

    setDeleting(true);
    setError('');

    try {
      let url: string;
      let body: any = {};

      if (deleteMode === 'version' && selectedDeleteRunIndex !== null) {
        // Delete specific version
        const run = session.runs[selectedDeleteRunIndex];
        url = `/api/v1/sessions/${sessionId}`;
        body = { run_id: run.id };
      } else {
        // Delete entire session
        url = `/api/v1/sessions/${sessionId}`;
      }

      const response = await fetchWithCSRF(url, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Failed to delete' }));
        throw new Error(errorData.error || 'Failed to delete');
      }

      const result = await response.json();

      // If session was deleted (either directly or because it was the last version), redirect to sessions list
      if (deleteMode === 'session' || result.session_deleted) {
        // Invalidate sessions cache to ensure fresh data on sessions list page
        queryClient.invalidateQueries({ queryKey: ['sessions'] });
        navigate('/sessions?success=Session deleted successfully');
      } else {
        // Refresh the session to show updated state
        await fetchSession();
        setShowDeleteDialog(false);
        setSuccessMessage('Version deleted successfully');
        setSuccessFading(false);
        // Start fade out after 4.5 seconds, then remove after animation completes
        setTimeout(() => setSuccessFading(true), 4500);
        setTimeout(() => setSuccessMessage(''), 5000);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete');
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div>
          <h1>Session Detail</h1>
          {session && (
            <p className={styles.sessionId}>
              <strong>Session ID:</strong> <code>{session.session_id}</code>
            </p>
          )}
        </div>
        <div className={styles.headerActions}>
          <button className={`${styles.btn} ${styles.btnShare}`} onClick={openShareDialog}>
            Share
          </button>
          <button className={`${styles.btn} ${styles.btnDanger}`} onClick={openDeleteDialog}>
            Delete
          </button>
        </div>
      </div>

      {successMessage && (
        <div className={`${styles.alert} ${styles.alertSuccess} ${successFading ? styles.alertFading : ''}`}>
          ✓ {successMessage}
        </div>
      )}

      {error ? (
        <div className={`${styles.alert} ${styles.alertError}`}>{error}</div>
      ) : loading ? (
        <div className={styles.card}>
          <p className={styles.loading}>Loading session...</p>
        </div>
      ) : session ? (
        <>
          <div className={styles.metaCard}>
            <div className={styles.metaItem}>
              <span className={styles.metaLabel}>First Seen:</span>
              <span className={styles.metaValue}>{formatRelativeTime(session.first_seen)}</span>
            </div>

            {/* Version selector dropdown (only show if multiple runs) */}
            {session.runs.length > 1 && (
              <div className={styles.metaItem}>
                <span className={styles.metaLabel}>Select Version:</span>
                <select
                  id="run-select"
                  value={selectedRunIndex}
                  onChange={(e) => setSelectedRunIndex(Number(e.target.value))}
                  className={styles.versionSelect}
                >
                  {session.runs.map((run, index) => {
                    const isLatestRun = session.runs.every(
                      (r) => new Date(run.end_timestamp) >= new Date(r.end_timestamp)
                    );
                    const isOldestRun = session.runs.every(
                      (r) => new Date(run.end_timestamp) <= new Date(r.end_timestamp)
                    );
                    const label = isLatestRun ? 'latest' : isOldestRun ? 'started' : 'updated';
                    return (
                      <option key={index} value={index}>
                        #{index + 1} {label} ({formatRelativeTime(run.end_timestamp)})
                      </option>
                    );
                  })}
                </select>
              </div>
            )}
          </div>

          {/* Display the selected run */}
          {selectedRun && <RunCard run={selectedRun} index={selectedRunIndex} />}
        </>
      ) : null}

      {/* Share Dialog Modal */}
      {showShareDialog && (
        <div className={styles.modalOverlay} onClick={() => setShowShareDialog(false)}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h2>Share Session</h2>
              <button className={styles.closeBtn} onClick={() => setShowShareDialog(false)}>
                ×
              </button>
            </div>

            <div className={styles.modalBody}>
              {createdShareURL ? (
                <div className={styles.successMessage}>
                  <h3>✓ Share Link Created</h3>
                  <div className={styles.shareUrlBox}>
                    <input type="text" readOnly value={createdShareURL} className={styles.shareUrlInput} />
                    <button className={`${styles.btn} ${styles.btnSm}`} onClick={() => copyToClipboard(createdShareURL)}>
                      Copy
                    </button>
                  </div>
                </div>
              ) : (
                <>
                  <div className={styles.formGroup}>
                    <label>
                      <input
                        type="radio"
                        checked={shareVisibility === 'public'}
                        onChange={() => setShareVisibility('public')}
                      />
                      <strong>Public</strong> - Anyone with link
                    </label>
                    <label>
                      <input
                        type="radio"
                        checked={shareVisibility === 'private'}
                        onChange={() => setShareVisibility('private')}
                      />
                      <strong>Private</strong> - Invite specific people
                    </label>
                  </div>

                  {shareVisibility === 'private' && (
                    <div className={styles.formGroup}>
                      <label>Invite by email:</label>
                      <div className={styles.emailInputGroup}>
                        <input
                          type="email"
                          value={newEmail}
                          onChange={(e) => setNewEmail(e.target.value)}
                          placeholder="email@example.com"
                          onKeyDown={(e) => e.key === 'Enter' && addEmail()}
                        />
                        <button className={`${styles.btn} ${styles.btnSm}`} onClick={addEmail}>
                          Add
                        </button>
                      </div>
                      {invitedEmails.length > 0 && (
                        <div className={styles.emailList}>
                          {invitedEmails.map((email) => (
                            <span key={email} className={styles.emailTag}>
                              {email}
                              <button className={styles.removeBtn} onClick={() => removeEmail(email)}>
                                ×
                              </button>
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  )}

                  <div className={styles.formGroup}>
                    <label>Expires:</label>
                    <select value={expiresInDays ?? 'null'} onChange={(e) => setExpiresInDays(e.target.value === 'null' ? null : Number(e.target.value))}>
                      <option value={1}>1 day</option>
                      <option value={7}>7 days</option>
                      <option value={30}>30 days</option>
                      <option value="null">Never</option>
                    </select>
                  </div>

                  <div className={styles.modalFooter}>
                    <button className={`${styles.btn} ${styles.btnPrimary}`} onClick={createShare}>
                      Create Share Link
                    </button>
                    <button className={`${styles.btn} ${styles.btnSecondary}`} onClick={() => setShowShareDialog(false)}>
                      Cancel
                    </button>
                  </div>
                </>
              )}

              <div className={styles.sharesList}>
                <h3>Active Shares</h3>
                {loadingShares ? (
                  <p>Loading...</p>
                ) : shares.length === 0 ? (
                  <p className={styles.empty}>No active shares</p>
                ) : (
                  shares.map((share) => (
                    <div key={share.share_token} className={styles.shareItem}>
                      <div className={styles.shareInfo}>
                        <span className={`${styles.visibilityBadge} ${styles[share.visibility]}`}>{share.visibility}</span>
                        {share.visibility === 'private' && share.invited_emails && (
                          <span className={styles.invited}>{share.invited_emails.join(', ')}</span>
                        )}
                        {share.expires_at ? (
                          <span className={styles.expires}>Expires: {formatDate(share.expires_at)}</span>
                        ) : (
                          <span className={styles.neverExpires}>Never expires</span>
                        )}
                      </div>
                      <button
                        className={`${styles.btn} ${styles.btnDanger} ${styles.btnSm}`}
                        onClick={() => revokeShare(share.share_token)}
                      >
                        Revoke
                      </button>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Delete Dialog Modal */}
      {showDeleteDialog && session && (
        <div className={styles.modalOverlay} onClick={() => !deleting && setShowDeleteDialog(false)}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h2>Delete Session</h2>
              <button
                className={styles.closeBtn}
                onClick={() => !deleting && setShowDeleteDialog(false)}
                disabled={deleting}
              >
                ×
              </button>
            </div>

            <div className={styles.modalBody}>
              {error && <div className={`${styles.alert} ${styles.alertError}`}>{error}</div>}

              <div className={styles.formGroup}>
                <p>What would you like to delete?</p>

                {session.runs.length > 1 ? (
                  <>
                    <label>
                      <input
                        type="radio"
                        checked={deleteMode === 'session'}
                        onChange={() => setDeleteMode('session')}
                        disabled={deleting}
                      />
                      <strong>Entire session</strong> - Delete all {session.runs.length} versions
                    </label>
                    <label>
                      <input
                        type="radio"
                        checked={deleteMode === 'version'}
                        onChange={() => {
                          setDeleteMode('version');
                          setSelectedDeleteRunIndex(0);
                        }}
                        disabled={deleting}
                      />
                      <strong>Specific version</strong> - Delete one version only
                    </label>

                    {deleteMode === 'version' && (
                      <div className={styles.formGroup} style={{ marginLeft: '1.5rem' }}>
                        <label>Select version to delete:</label>
                        <select
                          value={selectedDeleteRunIndex ?? 0}
                          onChange={(e) => setSelectedDeleteRunIndex(Number(e.target.value))}
                          className={styles.versionSelect}
                          disabled={deleting}
                        >
                          {session.runs.map((run, index) => {
                            const isLatestRun = session.runs.every(
                              (r) => new Date(run.end_timestamp) >= new Date(r.end_timestamp)
                            );
                            const isOldestRun = session.runs.every(
                              (r) => new Date(run.end_timestamp) <= new Date(r.end_timestamp)
                            );
                            const label = isLatestRun ? 'latest' : isOldestRun ? 'started' : 'updated';
                            return (
                              <option key={index} value={index}>
                                #{index + 1} {label} ({formatRelativeTime(run.end_timestamp)})
                              </option>
                            );
                          })}
                        </select>
                      </div>
                    )}
                  </>
                ) : (
                  <p>
                    This session has only one version. Deleting it will delete the entire session.
                  </p>
                )}
              </div>

              <div className={styles.warningMessage}>
                <strong>⚠️ Warning:</strong> This action cannot be undone. All associated files will be permanently deleted from storage.
              </div>

              <div className={styles.modalFooter}>
                <button
                  className={`${styles.btn} ${styles.btnDanger}`}
                  onClick={handleDelete}
                  disabled={deleting || (deleteMode === 'version' && selectedDeleteRunIndex === null)}
                >
                  {deleting ? 'Deleting...' : `Delete ${deleteMode === 'session' ? 'Session' : 'Version'}`}
                </button>
                <button
                  className={`${styles.btn} ${styles.btnSecondary}`}
                  onClick={() => setShowDeleteDialog(false)}
                  disabled={deleting}
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default SessionDetailPage;
