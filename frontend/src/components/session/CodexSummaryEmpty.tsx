// Empty-state placeholder for the Summary tab on Codex sessions.
// Codex analytics (cost, token usage, smart recap) are not yet computed
// server-side (CF-350); the tab still exists so the UI stays uniform across
// providers, but it explains the gap and points users to the Transcript tab.

import styles from './CodexSummaryEmpty.module.css';

export default function CodexSummaryEmpty() {
  return (
    <div className={styles.empty}>
      <div className={styles.icon} aria-hidden="true">
        ✨
      </div>
      <h3 className={styles.title}>Summary not yet available for Codex</h3>
      <p className={styles.body}>
        Analytics, smart recap, and per-session insights are still being built
        for Codex sessions. Open the Transcript tab to read the full
        conversation.
      </p>
    </div>
  );
}
