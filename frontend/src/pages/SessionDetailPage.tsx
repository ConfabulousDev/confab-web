import { useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { fetchWithCSRF } from '@/services/csrf';
import { sessionsAPI } from '@/services/api';
import { useDocumentTitle, useSuccessMessage, useLoadSession } from '@/hooks';
import type { SessionDetail } from '@/types';
import { SessionViewer } from '@/components/session';
import ShareDialog from '@/components/ShareDialog';
import styles from './SessionDetailPage.module.css';

function SessionDetailPage() {
  const { id: sessionId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const {
    message: successMessage,
    fading: successFading,
  } = useSuccessMessage();

  // Share dialog state
  const [showShareDialog, setShowShareDialog] = useState(false);

  // Delete dialog state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  // Fetch session using the shared hook
  const fetchSession = useCallback(async (): Promise<SessionDetail> => {
    if (!sessionId) throw new Error('No session ID');
    return sessionsAPI.get(sessionId);
  }, [sessionId]);

  const { session, setSession, loading, error } = useLoadSession({
    fetchSession,
    deps: [sessionId],
  });

  // Dynamic page title based on session (custom_title takes precedence)
  const pageTitle = session
    ? session.custom_title || session.summary || session.first_user_message || `Session ${session.external_id.substring(0, 8)}`
    : 'Session';
  useDocumentTitle(pageTitle);

  function openDeleteDialog() {
    setShowDeleteDialog(true);
    setDeleteError('');
  }

  async function handleDelete() {
    if (!sessionId || !session) return;

    setDeleting(true);
    setDeleteError('');

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

      queryClient.invalidateQueries({ queryKey: ['sessions'] });
      navigate('/sessions?success=Session deleted successfully');
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete');
    } finally {
      setDeleting(false);
    }
  }

  // Render loading state
  if (loading) {
    return (
      <div className={styles.container}>
        <div className={styles.loadingState}>
          <p>Loading session...</p>
        </div>
      </div>
    );
  }

  // Render error state
  if (error) {
    return (
      <div className={styles.container}>
        <div className={`${styles.alert} ${styles.alertError}`}>{error}</div>
      </div>
    );
  }

  // Render session viewer
  if (!session) {
    return null;
  }

  return (
    <div className={styles.container}>
      {successMessage && (
        <div className={`${styles.alert} ${styles.alertSuccess} ${successFading ? styles.alertFading : ''}`}>
          {successMessage}
        </div>
      )}

      <SessionViewer
        session={session}
        onShare={() => setShowShareDialog(true)}
        onDelete={openDeleteDialog}
        onSessionUpdate={(updatedSession) => {
          setSession(updatedSession);
          // Invalidate sessions list so updated title shows when navigating back
          queryClient.invalidateQueries({ queryKey: ['sessions'] });
        }}
        isOwner={true}
      />

      {/* Share Dialog Modal */}
      {sessionId && (
        <ShareDialog
          sessionId={sessionId}
          isOpen={showShareDialog}
          onClose={() => setShowShareDialog(false)}
        />
      )}

      {/* Delete Dialog Modal */}
      {showDeleteDialog && (
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
              {deleteError && <div className={`${styles.alert} ${styles.alertError}`}>{deleteError}</div>}

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
