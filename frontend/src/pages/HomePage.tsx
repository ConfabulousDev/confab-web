import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import Alert from '@/components/Alert';
import styles from './HomePage.module.css';

function HomePage() {
  useDocumentTitle('Confab');
  const { user, loading } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [authError, setAuthError] = useState<string | null>(null);

  useEffect(() => {
    // Check for auth error from OAuth callback
    const error = searchParams.get('error');
    const errorDescription = searchParams.get('error_description');

    if (error) {
      setAuthError(errorDescription || 'Authentication failed. Please try again.');
      // Clear error params from URL
      searchParams.delete('error');
      searchParams.delete('error_description');
      setSearchParams(searchParams, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  // Redirect logged-in users to sessions
  useEffect(() => {
    if (!loading && user) {
      navigate('/sessions', { replace: true });
    }
  }, [loading, user, navigate]);

  // Show nothing while loading or redirecting
  if (loading || user) {
    return null;
  }

  return (
    <div className={styles.container}>
      {authError && (
        <Alert variant="error" className={styles.errorAlert}>
          {authError}
        </Alert>
      )}

      {/* Hero Section */}
      <section className={styles.hero}>
        <h1 className={styles.title}>Share and review your Claude Code sessions</h1>
        <p className={styles.subtitle}>
          Capture, organize, and share your AI coding sessions with your team.
        </p>
      </section>

      {/* Value Props */}
      <section className={styles.features}>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>📤</div>
          <h3>Upload Sessions</h3>
          <p>Automatically sync your Claude Code transcripts to the cloud</p>
        </div>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>🔗</div>
          <h3>Share Securely</h3>
          <p>Create public or private links to share with teammates</p>
        </div>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>📖</div>
          <h3>Review Together</h3>
          <p>Browse full transcripts with tool calls and outputs</p>
        </div>
      </section>

      {/* How It Works */}
      <section className={styles.howItWorks}>
        <h2>How it works</h2>
        <div className={styles.steps}>
          <div className={styles.step}>
            <div className={styles.stepNumber}>1</div>
            <h4>Install the CLI</h4>
            <code>brew install anthropics/tap/confab</code>
          </div>
          <div className={styles.step}>
            <div className={styles.stepNumber}>2</div>
            <h4>Login & sync</h4>
            <code>confab login && confab push</code>
          </div>
          <div className={styles.step}>
            <div className={styles.stepNumber}>3</div>
            <h4>Share your session</h4>
            <p>Generate a link from the web UI</p>
          </div>
        </div>
      </section>
    </div>
  );
}

export default HomePage;
