import { Link } from 'react-router-dom';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import styles from './NotFoundPage.module.css';

function NotFoundPage() {
  useDocumentTitle('Not Found');
  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <h1 className={styles.title}>404</h1>
        <h2 className={styles.subtitle}>Page Not Found</h2>
        <p className={styles.message}>
          The page you're looking for doesn't exist or has been moved.
        </p>
        <div className={styles.actions}>
          <Link to="/" className={styles.homeBtn}>
            Go Home
          </Link>
          <button
            className={styles.backBtn}
            onClick={() => window.history.back()}
          >
            Go Back
          </button>
        </div>
      </div>
    </div>
  );
}

export default NotFoundPage;
