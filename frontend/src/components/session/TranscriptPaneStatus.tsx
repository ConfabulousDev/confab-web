// Shared loading / error placeholder for the transcript-tab panes
// (`ClaudeTranscriptPane`, `CodexTranscriptPane`). Both panes render the
// same centered-text status while their owner (`SessionViewer`) is still
// fetching the rollout, so the markup + CSS lives here once.

import styles from './TranscriptPaneStatus.module.css';

interface TranscriptPaneStatusProps {
  loading: boolean;
  error: string | null;
  /**
   * et0r D7: the open subagent thread's file isn't uploaded yet. Shown as a
   * retrying state (not an error) while `useTranscriptData` polls for it.
   */
  notSynced?: boolean;
}

/**
 * Returns the loading, not-synced, or error placeholder if any is set,
 * otherwise `null` — letting the caller fall through to its real content.
 */
export default function TranscriptPaneStatus({
  loading,
  error,
  notSynced = false,
}: TranscriptPaneStatusProps): React.ReactElement | null {
  if (loading) {
    return <div className={styles.loading}>Loading transcript...</div>;
  }
  if (notSynced) {
    return <div className={styles.loading}>Subagent transcript not synced yet — retrying</div>;
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
