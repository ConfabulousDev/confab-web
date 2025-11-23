import styles from './ErrorDisplay.module.css';

interface ErrorDisplayProps {
  message: string;
  retry?: () => void;
  details?: string;
}

function ErrorDisplay({ message, retry, details }: ErrorDisplayProps) {
  return (
    <div className={styles.errorDisplay}>
      <div className={styles.errorIcon}>⚠️</div>
      <p className={styles.errorMessage}>{message}</p>
      {details && <p className={styles.errorDetails}>{details}</p>}
      {retry && (
        <button className={styles.retryBtn} onClick={retry}>
          Try Again
        </button>
      )}
    </div>
  );
}

export default ErrorDisplay;
