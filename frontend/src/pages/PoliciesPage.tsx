import { Link } from 'react-router-dom';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import { LEGAL_DOCS } from '@/legal/docs';
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
          {LEGAL_DOCS.map((doc) => (
            <Link key={doc.slug} to={`/${doc.slug}`} className={styles.link}>
              <span className={styles.linkTitle}>{doc.title}</span>
              <span className={styles.linkDescription}>{doc.description}</span>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}

export default PoliciesPage;
