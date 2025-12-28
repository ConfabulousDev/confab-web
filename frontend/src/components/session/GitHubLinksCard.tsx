import { useState, useEffect, useCallback, useMemo } from 'react';
import { useVisibility } from '@/hooks';
import { githubLinksAPI, type GitHubLink } from '@/services/api';
import styles from './GitHubLinksCard.module.css';

// Polling interval for GitHub links (60 seconds)
const GITHUB_LINKS_POLL_INTERVAL_MS = 60000;

interface GitHubLinksCardProps {
  sessionId: string;
  isOwner: boolean;
  /** For Storybook: pass links directly instead of fetching from API */
  initialLinks?: GitHubLink[];
}

// Icons
const PRIcon = (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
    <path d="M1.5 3.25a2.25 2.25 0 1 1 3 2.122v5.256a2.251 2.251 0 1 1-1.5 0V5.372A2.25 2.25 0 0 1 1.5 3.25Zm5.677-.177L9.573.677A.25.25 0 0 1 10 .854V2.5h1A2.5 2.5 0 0 1 13.5 5v5.628a2.251 2.251 0 1 1-1.5 0V5a1 1 0 0 0-1-1h-1v1.646a.25.25 0 0 1-.427.177L7.177 3.427a.25.25 0 0 1 0-.354ZM3.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm0 9.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm8.25.75a.75.75 0 1 0 1.5 0 .75.75 0 0 0-1.5 0Z"/>
  </svg>
);

const CommitIcon = (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
    <path d="M11.93 8.5a4.002 4.002 0 0 1-7.86 0H.75a.75.75 0 0 1 0-1.5h3.32a4.002 4.002 0 0 1 7.86 0h3.32a.75.75 0 0 1 0 1.5Zm-1.43-.75a2.5 2.5 0 1 0-5 0 2.5 2.5 0 0 0 5 0Z"/>
  </svg>
);

const DeleteIcon = (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M18 6L6 18M6 6l12 12"/>
  </svg>
);

const PlusIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M12 5v14M5 12h14"/>
  </svg>
);

function GitHubLinksCard({ sessionId, isOwner, initialLinks }: GitHubLinksCardProps) {
  const isVisible = useVisibility();

  // State
  const [links, setLinks] = useState<GitHubLink[]>(initialLinks ?? []);
  const [loading, setLoading] = useState(initialLinks === undefined);
  const [error, setError] = useState<string | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newUrl, setNewUrl] = useState('');
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [deletingCommits, setDeletingCommits] = useState(false);

  const fetchLinks = useCallback(async (showLoading = true) => {
    if (initialLinks !== undefined) return;
    try {
      if (showLoading) setLoading(true);
      const response = await githubLinksAPI.list(sessionId);
      setLinks(response.links);
      setError(null);
    } catch (err) {
      if (showLoading) {
        console.error('Failed to fetch GitHub links:', err);
        setError('Failed to load');
      } else {
        console.warn('Failed to poll GitHub links:', err);
      }
    } finally {
      if (showLoading) setLoading(false);
    }
  }, [sessionId, initialLinks]);

  // Initial fetch
  useEffect(() => {
    fetchLinks();
  }, [fetchLinks]);

  // Poll when visible
  useEffect(() => {
    if (initialLinks !== undefined || !isVisible) return;
    const pollLinks = () => fetchLinks(false);
    const intervalId = setInterval(pollLinks, GITHUB_LINKS_POLL_INTERVAL_MS);
    return () => clearInterval(intervalId);
  }, [initialLinks, isVisible, fetchLinks]);

  const handleAddLink = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUrl.trim()) return;

    try {
      setAdding(true);
      await githubLinksAPI.create(sessionId, {
        url: newUrl.trim(),
        source: 'manual',
      });
      setNewUrl('');
      setShowAddForm(false);
      await fetchLinks();
    } catch (err) {
      console.error('Failed to add GitHub link:', err);
      setError(err instanceof Error ? err.message : 'Failed to add');
    } finally {
      setAdding(false);
    }
  };

  const handleDeleteLink = async (linkId: number) => {
    if (!window.confirm('Remove this GitHub link?')) return;
    try {
      setDeleting(linkId);
      await githubLinksAPI.delete(sessionId, linkId);
      await fetchLinks();
    } catch (err) {
      console.error('Failed to delete GitHub link:', err);
      setError('Failed to delete');
    } finally {
      setDeleting(null);
    }
  };

  // Get commit links sorted by created_at DESC
  const commitLinks = useMemo(() => {
    return links
      .filter((link) => link.link_type === 'commit')
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  }, [links]);

  const latestCommit = commitLinks.length > 0 ? commitLinks[0] : null;

  const prLinks = useMemo(() => {
    return links.filter((link) => link.link_type === 'pull_request');
  }, [links]);

  const handleDeleteAllCommits = async () => {
    if (commitLinks.length === 0) return;
    const shortShas = commitLinks.map((link) => link.ref.slice(0, 7));
    const message = `Remove all ${commitLinks.length} commit link${commitLinks.length > 1 ? 's' : ''}?\n\n${shortShas.join('\n')}`;
    if (!window.confirm(message)) return;

    try {
      setDeletingCommits(true);
      await githubLinksAPI.deleteByType(sessionId, 'commit');
      await fetchLinks();
    } catch (err) {
      console.error('Failed to delete commit links:', err);
      setError('Failed to delete');
    } finally {
      setDeletingCommits(false);
    }
  };

  const formatRef = (link: GitHubLink) => {
    if (link.link_type === 'pull_request') {
      return `#${link.ref}`;
    }
    return link.ref.slice(0, 7);
  };

  // Don't render if no links and not owner (can't add)
  if (!loading && links.length === 0 && !isOwner) {
    return null;
  }

  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <span>GitHub</span>
        {isOwner && !showAddForm && (
          <button
            className={styles.addButton}
            onClick={() => setShowAddForm(true)}
            title="Link to GitHub PR or commit"
          >
            {PlusIcon}
          </button>
        )}
      </div>

      <div className={styles.cardContent}>
        {error && <div className={styles.error}>{error}</div>}

        {showAddForm && (
          <form onSubmit={handleAddLink} className={styles.addForm}>
            <input
              type="url"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              placeholder="github.com/owner/repo/pull/123"
              className={styles.urlInput}
              disabled={adding}
              autoFocus
            />
            <div className={styles.addFormButtons}>
              <button
                type="submit"
                className={styles.submitButton}
                disabled={adding || !newUrl.trim()}
              >
                {adding ? '...' : 'Add'}
              </button>
              <button
                type="button"
                className={styles.cancelButton}
                onClick={() => {
                  setShowAddForm(false);
                  setNewUrl('');
                }}
                disabled={adding}
              >
                Cancel
              </button>
            </div>
          </form>
        )}

        {loading && links.length === 0 ? (
          <div className={styles.loading}>Loading...</div>
        ) : links.length === 0 && !showAddForm ? (
          <div className={styles.empty}>No linked PRs or commits</div>
        ) : (
          <>
            {/* PR Links */}
            {prLinks.map((link) => (
              <div key={link.id} className={styles.linkRow}>
                <a
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={styles.linkContent}
                  title={link.title || `${link.owner}/${link.repo}`}
                >
                  <span className={styles.linkIcon}>{PRIcon}</span>
                  <span className={styles.linkRef}>{formatRef(link)}</span>
                  <span className={styles.linkRepo}>{link.owner}/{link.repo}</span>
                </a>
                {isOwner && (
                  <button
                    className={styles.deleteButton}
                    onClick={() => handleDeleteLink(link.id)}
                    disabled={deleting === link.id}
                    title="Remove link"
                  >
                    {DeleteIcon}
                  </button>
                )}
              </div>
            ))}

            {/* Latest Commit */}
            {latestCommit && (
              <div className={styles.linkRow}>
                <a
                  href={latestCommit.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={styles.linkContent}
                  title={`${latestCommit.owner}/${latestCommit.repo}`}
                >
                  <span className={styles.linkIcon}>{CommitIcon}</span>
                  <span className={styles.linkRef}>{formatRef(latestCommit)}</span>
                  <span className={styles.linkRepo}>{latestCommit.owner}/{latestCommit.repo}</span>
                </a>
                {isOwner && (
                  <button
                    className={styles.deleteButton}
                    onClick={handleDeleteAllCommits}
                    disabled={deletingCommits}
                    title={`Remove ${commitLinks.length} commit link${commitLinks.length > 1 ? 's' : ''}`}
                  >
                    {DeleteIcon}
                  </button>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

export default GitHubLinksCard;
