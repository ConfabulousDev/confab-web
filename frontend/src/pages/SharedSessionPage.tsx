import { useState, useEffect } from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import type { SessionDetail, RunDetail } from '@/types';
import { formatDate, formatRelativeTime } from '@/utils';
import RunCard from '@/components/RunCard';
import styles from './SharedSessionPage.module.css';

type ErrorType = 'not_found' | 'expired' | 'forbidden' | 'general' | null;

function SharedSessionPage() {
  const { sessionId, token } = useParams<{ sessionId: string; token: string }>();
  const location = useLocation();
  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [errorType, setErrorType] = useState<ErrorType>(null);

  // Dynamic page title
  const pageTitle = session ? `Shared: ${session.external_id.substring(0, 8)}` : 'Shared Session';
  useDocumentTitle(pageTitle);

  useEffect(() => {
    async function loadSharedSession() {
      try {
        const response = await fetch(`/api/v1/sessions/${sessionId}/shared/${token}`, {
          credentials: 'include',
        });

        if (!response.ok) {
          if (response.status === 404) {
            setErrorType('not_found');
            setError('Share not found');
          } else if (response.status === 410) {
            setErrorType('expired');
            setError('This share link has expired');
          } else if (response.status === 401) {
            // Redirect to login, preserving the current URL to return after auth
            const intendedPath = location.pathname + location.search;
            window.location.href = `/auth/login?redirect=${encodeURIComponent(intendedPath)}`;
            return;
          } else if (response.status === 403) {
            setErrorType('forbidden');
            setError('You are not authorized to view this share');
          } else {
            setErrorType('general');
            setError('Failed to load shared session');
          }
          setLoading(false);
          return;
        }

        const data = await response.json();
        setSession(data);
        setLoading(false);
      } catch {
        setError('Failed to load shared session');
        setErrorType('general');
        setLoading(false);
      }
    }

    loadSharedSession();
  }, [sessionId, token, location.pathname, location.search]);

  function getErrorIcon(type: ErrorType): string {
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
      {/* Share Banner */}
      <div className={styles.shareBanner}>
        <span className={styles.shareIcon}>📤</span>
        <span>
          <strong>Shared Session</strong>
        </span>
      </div>

      {/* Session Header */}
      <div className={styles.header}>
        <div>
          <h1>Session Detail</h1>
          <p className={styles.sessionId}>
            <strong>Session ID:</strong> <code>{session.external_id}</code>
          </p>
        </div>
      </div>

      {/* Session Metadata */}
      {(() => {
        const firstRun = session.runs[0];
        const latestRun: RunDetail | undefined = firstRun
          ? session.runs.reduce((latest, run) =>
              new Date(run.end_timestamp) > new Date(latest.end_timestamp) ? run : latest
            , firstRun)
          : undefined;

        return (
          <>
            <div className={styles.metaCard}>
              <div className={styles.metaItem}>
                <span className={styles.metaLabel}>First Seen:</span>
                <span className={styles.metaValue}>{formatDate(session.first_seen)}</span>
              </div>
              <div className={styles.metaItem}>
                <span className={styles.metaLabel}>Last Updated:</span>
                <span className={styles.metaValue}>{latestRun && formatRelativeTime(latestRun.end_timestamp)}</span>
              </div>
            </div>

            {/* Display the latest run */}
            {latestRun && <RunCard run={latestRun} showGitInfo={false} shareToken={token} sessionId={session.id} />}
          </>
        );
      })()}
    </div>
  );
}

export default SharedSessionPage;
