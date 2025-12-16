import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '@/hooks';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import Alert from '@/components/Alert';
import Quickstart from '@/components/Quickstart';
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
              <>Please request access <a href={`mailto:${SUPPORT_EMAIL}?subject=${encodeURIComponent('Requesting access to Confabulous')}`}>here</a>.</>
            ) : (
              authError.message
            )}
          </Alert>
        )}

        <Quickstart />
      </div>
    </div>
  );
}

export default HomePage;
