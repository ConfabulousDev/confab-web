import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { fetchWithCSRF } from '@/services/csrf';
import { sessionsAPI, APIError } from '@/services/api';
import { useDocumentTitle, useSuccessMessage } from '@/hooks';
import type { SessionDetail } from '@/types';
import { formatRelativeTime } from '@/utils';
import SessionCard from '@/components/SessionCard';
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
  } = useSuccessMessage();

  // Share dialog state
  const [showShareDialog, setShowShareDialog] = useState(false);

  // Delete dialog state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);

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
      // Uses sessionsAPI which handles 401 globally via handleAuthFailure()
      const data = await sessionsAPI.get(sessionId);
      setSession(data);
    } catch (err) {
      // 401 is handled globally by the API client, so we only handle other errors here
      if (err instanceof APIError && err.status === 404) {
        setError('Session not found');
      } else {
        setError(err instanceof Error ? err.message : 'Failed to load session');
      }
    } finally {
      setLoading(false);
    }
  }

  function openDeleteDialog() {
    setShowDeleteDialog(true);
    setError('');
  }

  async function handleDelete() {
    if (!sessionId || !session) return;

    setDeleting(true);
    setError('');

    try {
      const url = `/api/v1/sessions/${sessionId}`;

      const response = await fetchWithCSRF(url, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Failed to delete' }));
        throw new Error(errorData.error || 'Failed to delete');
      }

      // Invalidate sessions cache to ensure fresh data on sessions list page
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
      navigate('/sessions?success=Session deleted successfully');
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
          {successMessage}
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
              <span className={styles.metaLabel}>First Synced:</span>
              <span className={styles.metaValue}>{formatRelativeTime(session.first_seen)}</span>
            </div>
            {session.last_sync_at && (
              <div className={styles.metaItem}>
                <span className={styles.metaLabel}>Latest Sync:</span>
                <span className={styles.metaValue}>{formatRelativeTime(session.last_sync_at)}</span>
              </div>
            )}
          </div>

          {/* Display the session details */}
          <SessionCard session={session} />
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

              <p>Are you sure you want to delete this session?</p>

              <div className={styles.warningMessage}>
                <strong>Warning:</strong> This action cannot be undone. All associated files will be permanently deleted from storage.
              </div>

              <div className={styles.modalFooter}>
                <button
                  className={`${styles.btn} ${styles.btnDanger}`}
                  onClick={handleDelete}
                  disabled={deleting}
                >
                  {deleting ? 'Deleting...' : 'Delete Session'}
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
