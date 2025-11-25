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
        <h1 className={styles.title}>Quantum flux harmonics for hyperdrive calibration</h1>
        <p className={styles.subtitle}>
          Synchronize your tachyon emitters across the subspace relay network.
        </p>
      </section>

      {/* Value Props */}
      <section className={styles.features}>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>🛸</div>
          <h3>Plasma Conduits</h3>
          <p>Route antimatter streams through your deflector array</p>
        </div>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>🌀</div>
          <h3>Warp Signatures</h3>
          <p>Encrypt your transponder codes with quantum entanglement</p>
        </div>
        <div className={styles.feature}>
          <div className={styles.featureIcon}>👽</div>
          <h3>Xenolinguistics</h3>
          <p>Parse alien transmissions with neural mesh decoders</p>
        </div>
      </section>

      {/* How It Works */}
      <section className={styles.howItWorks}>
        <h2>Initiation sequence</h2>
        <div className={styles.steps}>
          <div className={styles.step}>
            <div className={styles.stepNumber}>1</div>
            <h4>Engage the core</h4>
            <code>brew install zeta-reticuli/tap/confab</code>
          </div>
          <div className={styles.step}>
            <div className={styles.stepNumber}>2</div>
            <h4>Align crystals</h4>
            <code>confab calibrate && confab ignite</code>
          </div>
          <div className={styles.step}>
            <div className={styles.stepNumber}>3</div>
            <h4>Broadcast signal</h4>
            <p>Transmit via the orbital beacon array</p>
          </div>
        </div>
      </section>
    </div>
  );
}

export default HomePage;
