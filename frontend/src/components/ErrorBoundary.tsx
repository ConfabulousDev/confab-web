import { Component } from 'react';
import type { ReactNode } from 'react';
import styles from './ErrorBoundary.module.css';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

const CHUNK_RELOAD_KEY = 'chunk_reload_attempted';

/** Detect if error is a failed dynamic import (stale chunks after deployment) */
function isChunkLoadError(error: Error): boolean {
  const message = error.message.toLowerCase();
  return (
    message.includes('failed to fetch dynamically imported module') ||
    message.includes('loading chunk') ||
    (error.name === 'TypeError' && message.includes('failed to fetch'))
  );
}

class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo);

    // Auto-refresh on chunk load errors (stale assets after deployment)
    if (isChunkLoadError(error)) {
      const reloadKey = `${CHUNK_RELOAD_KEY}:${window.location.pathname}`;
      const alreadyAttempted = sessionStorage.getItem(reloadKey);

      if (!alreadyAttempted) {
        // Mark that we're attempting a reload to prevent infinite loops
        sessionStorage.setItem(reloadKey, 'true');
        // Refresh preserves the current URL (user's intended destination)
        window.location.reload();
        return;
      }
      // If reload already attempted, clear the flag and show error UI
      sessionStorage.removeItem(reloadKey);
    }
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className={styles.errorBoundary}>
          <div className={styles.errorCard}>
            <h2>Something went wrong</h2>
            <p className={styles.errorMessage}>
              {this.state.error?.message || 'An unexpected error occurred'}
            </p>
            <div className={styles.errorActions}>
              <button className={styles.retryBtn} onClick={this.handleReset}>
                Try Again
              </button>
              <button
                className={styles.homeBtn}
                onClick={() => (window.location.href = '/')}
              >
                Go Home
              </button>
            </div>
            {import.meta.env.DEV && this.state.error && (
              <details className={styles.errorDetails}>
                <summary>Error Details (Development Only)</summary>
                <pre>{this.state.error.stack}</pre>
              </details>
            )}
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
