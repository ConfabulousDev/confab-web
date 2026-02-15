import styles from './SessionEmptyState.module.css';

function SessionEmptyState() {
  return (
    <div className={styles.container}>
      <div className={styles.icon}>🔍</div>
      <p className={styles.message}>No sessions match the selected filters.</p>
      <p className={styles.hint}>Try adjusting or clearing your filters.</p>
    </div>
  );
}

export default SessionEmptyState;
