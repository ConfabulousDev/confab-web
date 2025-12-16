import styles from './SessionEmptyState.module.css';

export type SessionEmptyStateVariant = 'no-shared' | 'no-matches';

interface SessionEmptyStateProps {
  variant: SessionEmptyStateVariant;
}

function SessionEmptyState({ variant }: SessionEmptyStateProps) {
  if (variant === 'no-shared') {
    return (
      <div className={styles.container}>
        <div className={styles.icon}>📨</div>
        <p className={styles.message}>No sessions have been shared with you yet.</p>
      </div>
    );
  }

  // variant === 'no-matches'
  return (
    <div className={styles.container}>
      <div className={styles.icon}>🔍</div>
      <p className={styles.message}>No sessions match the selected filters.</p>
      <p className={styles.hint}>Try adjusting or clearing your filters.</p>
    </div>
  );
}

export default SessionEmptyState;
