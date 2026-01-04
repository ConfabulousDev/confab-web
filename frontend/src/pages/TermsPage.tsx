import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import styles from './TermsPage.module.css';

function TermsPage() {
  useDocumentTitle('Terms of Service');
  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <h1 className={styles.title}>Terms of Service</h1>

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Acceptance of Terms</h2>
          <p className={styles.text}>
            By accessing or using Confab, you agree to be bound by these Terms of Service.
            If you do not agree to these terms, please do not use the service.
          </p>
        </section>

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Description of Service</h2>
          <p className={styles.text}>
            Confab provides tools for syncing, viewing, and analyzing Claude Code session
            transcripts. The service is provided "as is" without warranties of any kind.
          </p>
        </section>

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>User Responsibilities</h2>
          <p className={styles.text}>
            You are responsible for maintaining the confidentiality of your API keys and
            account credentials. You agree not to share access tokens or attempt to access
            other users' data.
          </p>
        </section>

        <section id="privacy" className={styles.section}>
          <h2 className={styles.sectionTitle}>Data and Privacy</h2>
          <p className={styles.text}>
            Session transcripts you upload are stored securely and are only accessible to
            you and anyone you explicitly share them with. We do not sell or share your
            data with third parties. For questions about data handling, contact us at
            help@confabulous.dev.
          </p>
        </section>

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Limitation of Liability</h2>
          <p className={styles.text}>
            Confabulous Software LLC shall not be liable for any indirect, incidental,
            special, or consequential damages arising from your use of the service.
          </p>
        </section>

        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Trademark Attribution</h2>
          <p className={styles.text}>
            Claude and Claude Code are trademarks of Anthropic, PBC.
            Confabulous is not affiliated with, endorsed by, or sponsored by Anthropic.
          </p>
          <p className={styles.text}>
            GitHub and the GitHub logo are trademarks of GitHub, Inc.
          </p>
        </section>
      </div>
    </div>
  );
}

export default TermsPage;
