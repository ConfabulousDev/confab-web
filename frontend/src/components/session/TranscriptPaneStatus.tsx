// Shared loading / error placeholder for the transcript-tab panes
// (`ClaudeTranscriptPane`, `CodexTranscriptPane`). Both panes render the
// same centered-text status while their owner (`SessionViewer`) is still
// fetching the rollout, so the markup + CSS lives here once.

import styles from './TranscriptPaneStatus.module.css';

export interface TranscriptPaneStatusProps {
  loading: boolean;
  error: string | null;
}

/**
 * Returns the loading or error placeholder if either is set, otherwise
 * `null` — letting the caller fall through to its real content.
 */
export default function TranscriptPaneStatus({
  loading,
  error,
}: TranscriptPaneStatusProps): React.ReactElement | null {
  if (loading) {
    return <div className={styles.loading}>Loading transcript...</div>;
  }
  if (error) {
    return (
      <div className={styles.error}>
        <strong>Error:</strong> {error}
      </div>
    );
  }
  return null;
}
