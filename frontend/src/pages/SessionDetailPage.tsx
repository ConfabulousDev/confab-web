import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { fetchWithCSRF } from '@/services/csrf';
import { useDocumentTitle, useSuccessMessage } from '@/hooks';
import type { SessionDetail, RunDetail } from '@/types';
import { formatRelativeTime } from '@/utils';
import RunCard from '@/components/RunCard';
import ShareDialog from '@/components/ShareDialog';
import styles from './SessionDetailPage.module.css';

function SessionDetailPage() {
  const { id: sessionId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const {
    message: successMessage,
    fading: successFading,
    setMessage: setSuccessMessage,
  } = useSuccessMessage();

  // Share dialog state
  const [showShareDialog, setShowShareDialog] = useState(false);

  // Delete dialog state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleteMode, setDeleteMode] = useState<'session' | 'version'>('session');
  const [selectedDeleteRunIndex, setSelectedDeleteRunIndex] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Run selection state
  const [selectedRunIndex, setSelectedRunIndex] = useState(0);

  const selectedRun: RunDetail | undefined = session?.runs[selectedRunIndex];

  // Dynamic page title based on session
  const pageTitle = session ? `Session ${session.external_id.substring(0, 8)}` : 'Session';
  useDocumentTitle(pageTitle);

  useEffect(() => {
    if (sessionId) {
      fetchSession();
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
      const url = `/api/v1/sessions/${sessionId}`;
      const body: { run_id?: number } =
        deleteMode === 'version' && selectedDeleteRunIndex !== null
          ? { run_id: session.runs[selectedDeleteRunIndex].id }
          : {};

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
              <strong>Session ID:</strong> <code>{session.external_id}</code>
            </p>
          )}
        </div>
        <div className={styles.headerActions}>
          <button className={`${styles.btn} ${styles.btnShare}`} onClick={() => setShowShareDialog(true)}>
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
      {sessionId && (
        <ShareDialog
          sessionId={sessionId}
          isOpen={showShareDialog}
          onClose={() => setShowShareDialog(false)}
        />
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
