import type { GitInfo } from '@/types';
import { getRepoWebURL, getCommitURL } from '@/utils';
import styles from './GitInfoDisplay.module.css';

interface GitInfoDisplayProps {
  gitInfo: GitInfo;
}

function GitInfoDisplay({ gitInfo }: GitInfoDisplayProps) {
  return (
    <div className={styles.gitInfoSection}>
      <h4>Git Information</h4>
      <div className={styles.gitInfo}>
        {gitInfo.repo_url && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Repository:</span>
            {getRepoWebURL(gitInfo.repo_url) ? (
              <a
                href={getRepoWebURL(gitInfo.repo_url)!}
                target="_blank"
                rel="noopener noreferrer"
                className={`${styles.value} ${styles.link}`}
              >
                {gitInfo.repo_url}
              </a>
            ) : (
              <code className={styles.value}>{gitInfo.repo_url}</code>
            )}
          </div>
        )}

        {gitInfo.branch && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Branch:</span>
            <code className={styles.value}>{gitInfo.branch}</code>
            {gitInfo.is_dirty && <span className={styles.dirtyBadge}>⚠ Uncommitted changes</span>}
          </div>
        )}

        {gitInfo.commit_sha && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Commit:</span>
            {getCommitURL(gitInfo) ? (
              <a
                href={getCommitURL(gitInfo)!}
                target="_blank"
                rel="noopener noreferrer"
                className={`${styles.value} ${styles.link}`}
              >
                <code>{gitInfo.commit_sha.substring(0, 7)}</code>
              </a>
            ) : (
              <code className={styles.value}>{gitInfo.commit_sha.substring(0, 7)}</code>
            )}
          </div>
        )}

        {gitInfo.commit_message && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Message:</span>
            <span className={styles.value}>{gitInfo.commit_message}</span>
          </div>
        )}

        {gitInfo.author && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Author:</span>
            <span className={styles.value}>{gitInfo.author}</span>
          </div>
        )}
      </div>
    </div>
  );
}

export default GitInfoDisplay;
