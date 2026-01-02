import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import styles from './LegalPage.module.css';

function LegalPage() {
  useDocumentTitle('Legal');
  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <h1 className={styles.title}>Legal</h1>

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Trademark Attribution</h2>
          <p className={styles.text}>
            Claude and Claude Code are trademarks of Anthropic, PBC.
          </p>
          <p className={styles.text}>
            Confabulous is not affiliated with, endorsed by, or sponsored by Anthropic.
          </p>
        </section>
      </div>
    </div>
  );
}

export default LegalPage;
