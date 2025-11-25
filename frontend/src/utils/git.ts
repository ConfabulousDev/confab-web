// Git URL conversion utilities
import type { GitInfo } from '@/types';

/**
 * Convert a git repository URL to a web-browsable URL
 * Supports GitHub and GitLab SSH and HTTPS formats
 */
export function getRepoWebURL(repoUrl?: string): string | null {
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

/**
 * Get a web URL for a specific commit
 * Supports GitHub and GitLab
 */
export function getCommitURL(gitInfo?: GitInfo): string | null {
  const repoUrl = getRepoWebURL(gitInfo?.repo_url);
  if (!repoUrl || !gitInfo?.commit_sha) return null;

  if (repoUrl.includes('github.com')) {
    return `${repoUrl}/commit/${gitInfo.commit_sha}`;
  }
  if (repoUrl.includes('gitlab.com')) {
    return `${repoUrl}/-/commit/${gitInfo.commit_sha}`;
  }

  return null;
}
