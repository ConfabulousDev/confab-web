import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import Alert from '@/components/Alert';
import styles from './HomePage.module.css';

const SUPPORT_EMAIL = 'help@confabulous.dev';

interface AuthError {
  type: string;
  message: string;
}

// Helper to extract auth error from URL - used for lazy state initialization
function getInitialAuthError(searchParams: URLSearchParams): AuthError | null {
  const error = searchParams.get('error');
  if (error) {
    return {
      type: error,
      message: searchParams.get('error_description') || 'Authentication failed. Please try again.',
    };
  }
  return null;
}

// SVG Icons
const TerminalIcon = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <polyline points="4 17 10 11 4 5" />
    <line x1="12" y1="19" x2="20" y2="19" />
  </svg>
);

const ShareIcon = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
    <polyline points="16 6 12 2 8 6" />
    <line x1="12" y1="2" x2="12" y2="15" />
  </svg>
);

const SearchIcon = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
);

function HomePage() {
  useDocumentTitle('Confabulous');
  const { user, loading } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // Extract auth error from URL params into state on initial render.
  // Using lazy initialization ensures we capture the error before the URL is cleaned.
  const [authError] = useState(() => getInitialAuthError(searchParams));

  // Clear error params from URL after initial render
  useEffect(() => {
    if (searchParams.has('error')) {
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
    <div className={styles.wrapper}>
      <div className={styles.container}>
        {authError && (
          <Alert variant="error" className={styles.errorAlert}>
            {authError.type === 'access_denied' ? (
              <>Please request access <a href={`mailto:${SUPPORT_EMAIL}`}>here</a>.</>
            ) : (
              authError.message
            )}
          </Alert>
        )}

        {/* Hero Section */}
        <section className={styles.hero}>
        <h1 className={styles.title}>Record and share your Claude Code sessions</h1>
        <p className={styles.subtitle}>
          Capture coding conversations, review past sessions, and collaborate with your team.
        </p>
      </section>

      {/* Value Props */}
      <section className={styles.features}>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>{TerminalIcon}</div>
          <h3>Session Recording</h3>
          <p>Automatically capture your Claude Code sessions with full context and tool outputs</p>
        </div>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>{ShareIcon}</div>
          <h3>Easy Sharing</h3>
          <p>Share sessions with teammates via public or private links with optional expiration</p>
        </div>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>{SearchIcon}</div>
          <h3>Filter and Browse</h3>
          <p>Search through sessions by repository, branch, or content to find past conversations</p>
        </div>
      </section>

      {/* How It Works */}
      <section className={styles.howItWorks}>
        <h2>Get Started</h2>
        <div className={styles.steps}>
          <div className={styles.step}>
            <div className={styles.stepNumber}>1</div>
            <h4>Install the CLI</h4>
            <code>brew install confab-cli/tap/confab</code>
          </div>
          <div className={styles.step}>
            <div className={styles.stepNumber}>2</div>
            <h4>Login and configure</h4>
            <code>confab login && confab setup</code>
          </div>
          <div className={styles.step}>
            <div className={styles.stepNumber}>3</div>
            <h4>View your sessions</h4>
            <p>Sessions sync automatically as you code</p>
          </div>
        </div>
      </section>
      </div>
    </div>
  );
}

export default HomePage;
