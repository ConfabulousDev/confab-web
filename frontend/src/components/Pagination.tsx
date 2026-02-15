import styles from './Pagination.module.css';

interface PaginationProps {
  hasMore: boolean;
  canGoPrev: boolean;
  onNext: () => void;
  onPrev: () => void;
}

function Pagination({ hasMore, canGoPrev, onNext, onPrev }: PaginationProps) {
  if (!hasMore && !canGoPrev) return null;

  return (
    <div className={styles.container}>
      <button
        className={styles.navBtn}
        onClick={onPrev}
        disabled={!canGoPrev}
        aria-label="Previous page"
      >
        Prev
      </button>
      <button
        className={styles.navBtn}
        onClick={onNext}
        disabled={!hasMore}
        aria-label="Next page"
      >
        Next
      </button>
    </div>
  );
}

export default Pagination;
