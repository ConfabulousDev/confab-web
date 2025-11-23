import type { GitInfo } from '@/types';
import styles from './GitInfoDisplay.module.css';

interface GitInfoDisplayProps {
  gitInfo: GitInfo;
}

function getRepoWebURL(repoUrl?: string): string | null {
  if (!repoUrl) return null;

  // Convert SSH URLs to HTTPS
  if (repoUrl.startsWith('git@github.com:')) {
    return repoUrl.replace('git@github.com:', 'https://github.com/').replace(/\.git$/, '');
  }
  if (repoUrl.startsWith('git@gitlab.com:')) {
    return repoUrl.replace('git@gitlab.com:', 'https://gitlab.com/').replace(/\.git$/, '');
  }

  // HTTPS URLs
  if (repoUrl.startsWith('https://github.com/') || repoUrl.startsWith('https://gitlab.com/')) {
    return repoUrl.replace(/\.git$/, '');
  }

  return null;
}

function getCommitURL(gitInfo: GitInfo): string | null {
  const repoUrl = getRepoWebURL(gitInfo.repo_url);
  if (!repoUrl || !gitInfo.commit_sha) return null;

  if (repoUrl.includes('github.com')) {
    return `${repoUrl}/commit/${gitInfo.commit_sha}`;
  }
  if (repoUrl.includes('gitlab.com')) {
    return `${repoUrl}/-/commit/${gitInfo.commit_sha}`;
  }

  return null;
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
