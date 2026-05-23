// CF-483: Persistent top-of-page strip that renders when DEMO_IDENTITY_EMAIL
// is configured on the backend. Above the nav. Not dismissible.

import { getDemoIdentity } from '@/utils/demoIdentity';
import styles from './DemoBanner.module.css';

const SELF_HOST_URL =
  'https://github.com/ConfabulousDev/confab-web/blob/main/SELF-HOSTING.md';
const REPO_URL = 'https://github.com/ConfabulousDev/confab-web';

function DemoBanner() {
  const email = getDemoIdentity();
  if (!email) return null;

  return (
    <div className={styles.banner} role="status">
      <div className={styles.left}>
        Read-only demo — viewing as <span className={styles.email}>{email}</span>
      </div>
      <div className={styles.links}>
        <a href={SELF_HOST_URL} target="_blank" rel="noopener noreferrer">
          Self-host
        </a>
        <a href={REPO_URL} target="_blank" rel="noopener noreferrer">
          GitHub
        </a>
      </div>
    </div>
  );
}

export default DemoBanner;
export { SELF_HOST_URL, REPO_URL };
