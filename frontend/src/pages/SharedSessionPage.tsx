import { useCallback } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useDocumentTitle, useLoadSession } from '@/hooks';
import type { SessionErrorType } from '@/hooks';
import type { SessionDetail } from '@/types';
import { SessionViewer } from '@/components/session';
import styles from './SharedSessionPage.module.css';

function getErrorIcon(type: SessionErrorType): string {
  switch (type) {
    case 'not_found':
      return '🔍';
    case 'expired':
      return '⏰';
    case 'forbidden':
      return '🚫';
    default:
      return '⚠️';
  }
}

function SharedSessionPage() {
  const { sessionId, token } = useParams<{ sessionId: string; token: string }>();
  const [searchParams] = useSearchParams();

  // Get email params from URL
  const expectedEmail = searchParams.get('email');
  const emailMismatch = searchParams.get('email_mismatch') === '1';
  const mismatchExpected = searchParams.get('expected');
  const mismatchActual = searchParams.get('actual');

  const fetchSharedSession = useCallback(async (): Promise<SessionDetail> => {
    const response = await fetch(`/api/v1/sessions/${sessionId}/shared/${token}`, {
      credentials: 'include',
    });

    if (!response.ok) {
      // Throw response-like object so hook can handle status codes
      throw { status: response.status };
    }

    return response.json();
  }, [sessionId, token]);

  const handleAuthRequired = useCallback((redirectPath: string) => {
    // Include email param in login redirect if present
    let loginUrl = `/auth/login?redirect=${encodeURIComponent(redirectPath)}`;
    if (expectedEmail) {
      loginUrl += `&email=${encodeURIComponent(expectedEmail)}`;
    }
    window.location.href = loginUrl;
  }, [expectedEmail]);

  const { session, loading, error, errorType } = useLoadSession({
    fetchSession: fetchSharedSession,
    onAuthRequired: handleAuthRequired,
    deps: [sessionId, token],
  });

  // Dynamic page title
  const pageTitle = session ? `Shared: ${session.external_id.substring(0, 8)}` : 'Shared Session';
  useDocumentTitle(pageTitle);

  if (loading) {
    return (
      <div className={styles.container}>
        <div className={styles.loading}>Loading shared session...</div>
      </div>
    );
  }

  // Handle email mismatch - user logged in with wrong email
  if (emailMismatch && mismatchExpected && mismatchActual) {
    const handleLogoutAndRetry = () => {
      // Redirect to logout, which will redirect back to login with email hint
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

  if (error) {
    // Show specific message for forbidden errors when we know the expected email
    const showEmailHint = errorType === 'forbidden' && expectedEmail;

    return (
      <div className={styles.container}>
        <div className={styles.errorContainer}>
          <div className={styles.errorIcon}>{getErrorIcon(errorType)}</div>
          <h2>{error}</h2>
          {errorType === 'forbidden' && !showEmailHint && <p>This share is only accessible to invited users.</p>}
          {showEmailHint && (
            <p>This share was sent to <strong>{expectedEmail}</strong>. Please sign in with that account.</p>
          )}
          {errorType === 'expired' && <p>Please request a new share link from the session owner.</p>}
        </div>
      </div>
    );
  }

  if (!session) {
    return null;
  }

  // Owner viewing their own share link gets isOwner=true but isShared=true for UX
  const isOwnerViewingShare = session.is_owner === true;

  return (
    <div className={styles.container}>
      <SessionViewer
        session={session}
        shareToken={token}
        isOwner={isOwnerViewingShare}
        isShared={true}
      />
    </div>
  );
}

export default SharedSessionPage;
