import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import { isDemoViewer } from '@/utils/demoIdentity';
import CTALinks from '@/components/CTALinks';
import HeroCards from '@/components/HeroCards';
import styles from './HomePage.module.css';

function HomePage() {
  useDocumentTitle('Confabulous');
  const { user, loading, serverError } = useAuth();
  const navigate = useNavigate();

  // Redirect logged-in users to sessions (skip if server is down).
  // Demo identity skips the ?owner= pre-filter — see Header.tsx for the
  // rationale; same fix, same gate.
  useEffect(() => {
    if (!loading && user && !serverError) {
      const target = user.email && !isDemoViewer(user.email)
        ? `/sessions?owner=${encodeURIComponent(user.email)}`
        : '/sessions';
      navigate(target, { replace: true });
    }
  }, [loading, user, serverError, navigate]);

  // Show nothing while loading or redirecting
  if (loading || user) {
    return null;
  }

  return (
    <div className={styles.wrapper}>
      <div className={styles.container}>
        <div className={styles.hero}>
          <h1 className={styles.headline}>Understand your AI coding sessions</h1>
          <ul className={styles.bullets}>
            <li>Open source and self-hostable. Maintain data sovereignty.</li>
            <li>Claude Code · OpenAI Codex</li>
          </ul>
        </div>

        <CTALinks />
        <HeroCards />
        <CTALinks />
      </div>
    </div>
  );
}

export default HomePage;
