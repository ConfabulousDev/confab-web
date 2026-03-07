import { useAutoRetry } from '@/hooks/useAutoRetry';
import styles from './ServerError.module.css';

const MAX_ATTEMPTS = 10;

interface ServerErrorProps {
  message: string | null;
  onRetry: () => Promise<unknown>;
}

function getStatusText(
  isRetrying: boolean,
  exhausted: boolean,
  countdown: number,
  attempt: number,
): string | null {
  if (isRetrying) return 'Retrying...';
  if (exhausted) return 'Automatic retries exhausted';
  if (countdown > 0) return `Retrying in ${countdown}s... (attempt ${attempt + 1} of ${MAX_ATTEMPTS})`;
  return null;
}

function ServerError({ message, onRetry }: ServerErrorProps) {
  const { countdown, attempt, isRetrying, exhausted } = useAutoRetry(onRetry, {
    maxAttempts: MAX_ATTEMPTS,
    initialDelay: 10_000,
    maxDelay: 60_000,
    enabled: true,
  });

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <h2>Server Unavailable</h2>
        <p className={styles.subtitle}>This is usually temporary</p>
        <p className={styles.message}>
          {message || 'Unable to reach the server. It may be restarting.'}
        </p>
        <p className={styles.status}>
          {getStatusText(isRetrying, exhausted, countdown, attempt)}
        </p>
        <button
          className={styles.retryBtn}
          onClick={onRetry}
          disabled={isRetrying}
        >
          Retry Now
        </button>
      </div>
    </div>
  );
}

export default ServerError;
