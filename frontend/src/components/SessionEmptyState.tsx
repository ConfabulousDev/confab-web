import { DOCS_URL } from '@/utils/externalLinks';
import styles from './SessionEmptyState.module.css';

function SessionEmptyState() {
  return (
    <div className={styles.container}>
      <div className={styles.icon}>🔍</div>
      <p className={styles.message}>No sessions match the selected filters.</p>
      <p className={styles.hint}>Try adjusting or clearing your filters.</p>
      {/* CF-571: nudge newcomers toward the docs site. */}
      <p className={styles.docsHint}>
        New to Confabulous?{' '}
        <a href={DOCS_URL} target="_blank" rel="noopener noreferrer" className={styles.docsLink}>
          Read the docs
        </a>
      </p>
    </div>
  );
}

export default SessionEmptyState;
