import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import termsHtml from './terms-content.html?raw';
import styles from './TermsPage.module.css';

function TermsPage() {
  useDocumentTitle('Terms of Service');
  return (
    <div className={styles.wrapper}>
      <div className={styles.container}>
        <div className={styles.content}>
        <div
          className={styles.termsContent}
          dangerouslySetInnerHTML={{ __html: termsHtml }}
        />

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Trademark Attribution</h2>
          <p className={styles.text}>
            Claude and Claude Code are trademarks of Anthropic, PBC. Confabulous Software
            LLC is not affiliated with, endorsed by, or sponsored by Anthropic.
          </p>
          <p className={styles.text}>
            GitHub and the GitHub logo are trademarks of GitHub, Inc.
          </p>
        </section>
        </div>
      </div>
    </div>
  );
}

export default TermsPage;
