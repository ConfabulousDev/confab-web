import styles from './LoadingSkeleton.module.css';

interface LoadingSkeletonProps {
  variant?: 'text' | 'card' | 'list';
  count?: number;
}

function LoadingSkeleton({ variant = 'card', count = 1 }: LoadingSkeletonProps) {
  if (variant === 'text') {
    return (
      <div className={styles.textSkeleton}>
        {Array.from({ length: count }).map((_, i) => (
          <div key={i} className={styles.textLine} />
        ))}
      </div>
    );
  }

  if (variant === 'list') {
    return (
      <div className={styles.listSkeleton}>
        {Array.from({ length: count }).map((_, i) => (
          <div key={i} className={styles.listItem}>
            <div className={styles.skeletonBlock} style={{ width: '60%' }} />
            <div className={styles.skeletonBlock} style={{ width: '40%' }} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className={styles.cardSkeleton}>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className={styles.card}>
          <div className={styles.cardHeader}>
            <div className={styles.skeletonBlock} style={{ width: '200px', height: '24px' }} />
            <div className={styles.skeletonBlock} style={{ width: '100px', height: '20px' }} />
          </div>
          <div className={styles.cardBody}>
            <div className={styles.skeletonBlock} style={{ width: '100%', height: '16px' }} />
            <div className={styles.skeletonBlock} style={{ width: '80%', height: '16px' }} />
            <div className={styles.skeletonBlock} style={{ width: '90%', height: '16px' }} />
          </div>
        </div>
      ))}
    </div>
  );
}

export default LoadingSkeleton;
