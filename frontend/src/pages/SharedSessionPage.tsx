import { useCallback } from 'react';
import { useParams } from 'react-router-dom';
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
    window.location.href = `/auth/login?redirect=${encodeURIComponent(redirectPath)}`;
  }, []);

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

  if (error) {
    return (
      <div className={styles.container}>
        <div className={styles.errorContainer}>
          <div className={styles.errorIcon}>{getErrorIcon(errorType)}</div>
          <h2>{error}</h2>
          {errorType === 'forbidden' && <p>This share is only accessible to invited users.</p>}
          {errorType === 'expired' && <p>Please request a new share link from the session owner.</p>}
        </div>
      </div>
    );
  }

  if (!session) {
    return null;
  }

  return (
    <div className={styles.container}>
      <SessionViewer
        session={session}
        shareToken={token}
        isOwner={false}
        isShared={true}
      />
    </div>
  );
}

export default SharedSessionPage;
