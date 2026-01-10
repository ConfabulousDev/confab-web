import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import styles from './PoliciesPage.module.css';

function PoliciesPage() {
  useDocumentTitle('Policies');
  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <h1 className={styles.title}>Policies</h1>
        <p className={styles.subtitle}>
          Legal documents and policies for Confabulous.dev.
        </p>

        <div className={styles.links}>
          <a
            href="https://app.termly.io/policy-viewer/policy.html?policyUUID=69001385-5934-4a9f-9ade-ca93873b3e6c"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.link}
          >
            <span className={styles.linkTitle}>Terms of Service</span>
            <span className={styles.linkDescription}>
              Terms and conditions for using Confabulous.dev
            </span>
          </a>

          <a
            href="https://app.termly.io/policy-viewer/policy.html?policyUUID=7366762a-c58a-4a7a-9cf0-f39620707a60"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.link}
          >
            <span className={styles.linkTitle}>Privacy Notice</span>
            <span className={styles.linkDescription}>
              How we collect, use, and protect your data
            </span>
          </a>

          <a
            href="https://app.termly.io/policy-viewer/policy.html?policyUUID=fec4df5c-7eb9-4687-9356-218047726cae"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.link}
          >
            <span className={styles.linkTitle}>Cookie Policy</span>
            <span className={styles.linkDescription}>
              How we use cookies and similar technologies
            </span>
          </a>

          <a
            href="https://app.termly.io/policy-viewer/policy.html?policyUUID=7a3b8843-fe1c-43e3-8d02-88cb537a8874"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.link}
          >
            <span className={styles.linkTitle}>Disclaimer</span>
            <span className={styles.linkDescription}>
              Important notices and limitations of liability
            </span>
          </a>

          <a
            href="https://app.termly.io/policy-viewer/policy.html?policyUUID=2fb5c0b3-3458-48c5-8fcd-4803d17af25d"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.link}
          >
            <span className={styles.linkTitle}>Acceptable Use Policy</span>
            <span className={styles.linkDescription}>
              Guidelines for appropriate use of our services
            </span>
          </a>
        </div>
      </div>
    </div>
  );
}

export default PoliciesPage;
