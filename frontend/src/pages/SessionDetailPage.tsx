import { useState, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { fetchWithCSRF } from '@/services/csrf';
import { sessionsAPI } from '@/services/api';
import { useAuth, useDocumentTitle, useSuccessMessage, useLoadSession } from '@/hooks';
import type { SessionDetail } from '@/types';
import { getErrorIcon, getErrorDescription } from '@/utils/sessionErrors';
import { SessionViewer, type ViewTab } from '@/components/session';
import ShareDialog from '@/components/ShareDialog';
import styles from './SessionDetailPage.module.css';

function isValidViewTab(value: string | null): value is ViewTab {
  return value === 'summary' || value === 'transcript';
}

function SessionDetailPage() {
  const { id: sessionId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const { isAuthenticated } = useAuth();
  const {
    message: successMessage,
    fading: successFading,
  } = useSuccessMessage();

  // Tab state from URL params (defaults to 'summary')
  const tabParam = searchParams.get('tab');
  const activeTab: ViewTab = isValidViewTab(tabParam) ? tabParam : 'summary';

  const handleTabChange = useCallback((tab: ViewTab) => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev);
      if (tab === 'summary') {
        next.delete('tab');
      } else {
        next.set('tab', tab);
      }
      return next;
    }, { replace: true });
  }, [setSearchParams]);

  // Email mismatch query params (for recipient shares with wrong account)
  const emailMismatch = searchParams.get('email_mismatch') === '1';
  const mismatchExpected = searchParams.get('expected');
  const mismatchActual = searchParams.get('actual');

  // Share dialog state
  const [showShareDialog, setShowShareDialog] = useState(false);

  // Delete dialog state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  // Fetch session using the shared hook (canonical endpoint supports all access types)
  const fetchSession = useCallback(async (): Promise<SessionDetail> => {
    if (!sessionId) throw new Error('No session ID');
    return sessionsAPI.get(sessionId);
  }, [sessionId]);

  const { session, setSession, loading, error, errorType } = useLoadSession({
    fetchSession,
    deps: [sessionId],
  });

  // Determine if viewer is owner (from backend response, defaults to false for non-owners)
  const isOwner = session?.is_owner ?? false;

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

  // Handle email mismatch - user logged in with wrong email for recipient share
  if (emailMismatch && mismatchExpected && mismatchActual) {
    const handleLogoutAndRetry = () => {
      const currentPath = window.location.pathname + window.location.search.replace(/[&?]email_mismatch=1.*/, '');
      const loginUrl = `/auth/login?redirect=${encodeURIComponent(currentPath)}&email=${encodeURIComponent(mismatchExpected)}`;
      window.location.href = `/auth/logout?redirect=${encodeURIComponent(loginUrl)}`;
    };

    return (
      <div className={styles.container}>
        <div className={styles.errorContainer}>
          <div className={styles.errorIcon}>🔐</div>
          <h2>Wrong Account</h2>
          <p>This share was sent to:</p>
          <p className={styles.email}><strong>{mismatchExpected}</strong></p>
          <p>You&apos;re signed in as:</p>
          <p className={styles.email}><strong>{mismatchActual}</strong></p>
          <button className={styles.retryButton} onClick={handleLogoutAndRetry}>
            Sign in with correct account
          </button>
        </div>
      </div>
    );
  }

  // Render error state with appropriate icon and message
  if (error) {
    // For auth_required, show login prompt with redirect back to this page
    const handleSignIn = () => {
      const currentPath = window.location.pathname + window.location.search;
      window.location.href = `/auth/login?redirect=${encodeURIComponent(currentPath)}`;
    };

    const description = getErrorDescription(errorType);
    // Show login hint for not_found errors when user is not authenticated
    const showLoginHint = errorType === 'not_found' && !isAuthenticated;

    return (
      <div className={styles.container}>
        <div className={styles.errorContainer}>
          <div className={styles.errorIcon}>{getErrorIcon(errorType)}</div>
          <h2>{error}</h2>
          {description && <p>{description}</p>}
          {showLoginHint && <p>If you have access to this session, try signing in.</p>}
          {(errorType === 'auth_required' || showLoginHint) && (
            <button className={styles.retryButton} onClick={handleSignIn}>
              Sign in
            </button>
          )}
        </div>
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
        onShare={isOwner ? () => setShowShareDialog(true) : undefined}
        onDelete={isOwner ? openDeleteDialog : undefined}
        onSessionUpdate={isOwner ? (updatedSession) => {
          setSession(updatedSession);
          // Invalidate sessions list so updated title shows when navigating back
          queryClient.invalidateQueries({ queryKey: ['sessions'] });
        } : undefined}
        isOwner={isOwner}
        isShared={!isOwner}
        activeTab={activeTab}
        onTabChange={handleTabChange}
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
