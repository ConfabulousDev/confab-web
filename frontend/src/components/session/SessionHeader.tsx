import type { GitInfo } from '@/types';
import { useCopyToClipboard } from '@/hooks';
import styles from './SessionHeader.module.css';

interface SessionHeaderProps {
  title?: string;
  externalId: string;
  model?: string;
  durationMs?: number;
  sessionDate?: Date;
  gitInfo?: GitInfo | null;
  onShare?: () => void;
  onDelete?: () => void;
  isOwner?: boolean;
}

/**
 * Format duration in a human-readable way
 */
function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return `${hours}h ${remainingMinutes}m`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
}

/**
 * Format date for display
 */
function formatDate(date: Date): string {
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Extract a short model name from the full model string
 * Examples:
 *   "claude-3-5-sonnet-20241022" -> "claude-3.5-sonnet"
 *   "claude-opus-4-20250514" -> "claude-opus-4"
 *   "claude-sonnet-4-5-20250514" -> "claude-sonnet-4.5"
 *   "opus" -> "opus"
 */
function formatModelName(model: string): string {
  const lowerModel = model.toLowerCase();

  // Look for variant names
  const variants = ['sonnet', 'opus', 'haiku'];
  const foundVariant = variants.find(v => lowerModel.includes(v));

  if (!foundVariant) {
    // No known variant, return as-is or truncated
    const parts = model.split('-');
    if (parts.length > 3) {
      return parts.slice(0, 3).join('-');
    }
    return model;
  }

  // Try to extract version number - handles both "claude-3-5-sonnet" and "claude-opus-4"
  // Pattern 1: claude-X-Y-variant (e.g., claude-3-5-sonnet)
  const preVariantMatch = model.match(/claude-(\d+(?:-\d+)?)-\w+/i);
  // Pattern 2: claude-variant-X (e.g., claude-opus-4)
  const postVariantMatch = model.match(new RegExp(`${foundVariant}-(\\d+(?:-\\d+)?)`, 'i'));

  let version = '';
  if (preVariantMatch?.[1]) {
    version = preVariantMatch[1].replace('-', '.');
  } else if (postVariantMatch?.[1]) {
    version = postVariantMatch[1].replace('-', '.');
  }

  // Build the display name
  if (version) {
    return `claude-${foundVariant}-${version}`;
  }
  return `claude-${foundVariant}`;
}

/**
 * Extract repo name from URL
 * e.g., "https://github.com/user/repo.git" -> "user/repo"
 */
function formatRepoName(repoUrl: string): string {
  // Handle GitHub/GitLab URLs
  const match = repoUrl.match(/(?:github\.com|gitlab\.com)[/:]([^/]+\/[^/]+?)(?:\.git)?$/i);
  if (match?.[1]) {
    return match[1];
  }
  // Fallback: just show last two path segments
  const parts = repoUrl.replace(/\.git$/, '').split('/');
  if (parts.length >= 2) {
    return `${parts[parts.length - 2]}/${parts[parts.length - 1]}`;
  }
  return repoUrl;
}

function SessionHeader({
  title,
  externalId,
  model,
  durationMs,
  sessionDate,
  gitInfo,
  onShare,
  onDelete,
  isOwner = true,
}: SessionHeaderProps) {
  const { copy, copied } = useCopyToClipboard();
  const displayTitle = title || `Session ${externalId.substring(0, 8)}`;

  return (
    <header className={styles.header}>
      <div className={styles.titleSection}>
        <div className={styles.titleRow}>
          <h1 className={styles.title}>{displayTitle}</h1>
          <button
            className={styles.copyIdBtn}
            onClick={() => copy(externalId)}
            title={copied ? 'Copied!' : 'Copy Claude Code session id'}
            aria-label="Copy Claude Code session id"
          >
            {copied ? (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            ) : (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
            )}
          </button>
        </div>
        <div className={styles.metadata}>
          {/* Git repo */}
          {gitInfo?.repo_url && (
            <span className={styles.metaItem}>
              <span className={styles.metaIcon}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                </svg>
              </span>
              <span className={styles.metaValue}>{formatRepoName(gitInfo.repo_url)}</span>
            </span>
          )}

          {/* Git branch */}
          {gitInfo?.branch && (
            <span className={styles.metaItem}>
              <span className={styles.metaIcon}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <line x1="6" y1="3" x2="6" y2="15" />
                  <circle cx="18" cy="6" r="3" />
                  <circle cx="6" cy="18" r="3" />
                  <path d="M18 9a9 9 0 0 1-9 9" />
                </svg>
              </span>
              <span className={styles.metaValue}>{gitInfo.branch}</span>
            </span>
          )}

          {/* Git commit */}
          {gitInfo?.commit_sha && (
            <span className={styles.metaItem}>
              <span className={styles.metaIcon}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="4" />
                  <line x1="1.05" y1="12" x2="7" y2="12" />
                  <line x1="17.01" y1="12" x2="22.96" y2="12" />
                </svg>
              </span>
              <span className={styles.metaValue}>{gitInfo.commit_sha.substring(0, 7)}</span>
            </span>
          )}

          {/* Model */}
          {model && (
            <span className={styles.metaItem}>
              <span className={styles.metaIcon}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="2" y="6" width="20" height="12" rx="2" />
                  <path d="M12 12h.01" />
                  <path d="M17 12h.01" />
                  <path d="M7 12h.01" />
                </svg>
              </span>
              <span className={styles.metaValue}>{formatModelName(model)}</span>
            </span>
          )}

          {/* Duration */}
          {durationMs !== undefined && durationMs > 0 && (
            <span className={styles.metaItem}>
              <span className={styles.metaIcon}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
              </span>
              <span className={styles.metaValue}>{formatDuration(durationMs)}</span>
            </span>
          )}

          {/* Date */}
          {sessionDate && (
            <span className={styles.metaItem}>
              <span className={styles.metaIcon}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
                  <line x1="16" y1="2" x2="16" y2="6" />
                  <line x1="8" y1="2" x2="8" y2="6" />
                  <line x1="3" y1="10" x2="21" y2="10" />
                </svg>
              </span>
              <span className={styles.metaValue}>{formatDate(sessionDate)}</span>
            </span>
          )}
        </div>
      </div>

      {isOwner && (
        <div className={styles.actions}>
          {onShare && (
            <button className={styles.btnShare} onClick={onShare}>
              Share
            </button>
          )}
          {onDelete && (
            <button className={styles.btnDelete} onClick={onDelete}>
              Delete
            </button>
          )}
        </div>
      )}
    </header>
  );
}

export default SessionHeader;
