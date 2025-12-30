import { useState, useEffect, useCallback, useMemo } from 'react';
import { useVisibility } from '@/hooks';
import { githubLinksAPI, type GitHubLink } from '@/services/api';
import { GitHubIcon } from '@/components/icons';
import styles from './GitHubLinksCard.module.css';

// Polling interval for GitHub links (60 seconds)
const GITHUB_LINKS_POLL_INTERVAL_MS = 60000;

interface GitHubLinksCardProps {
  sessionId: string;
  isOwner: boolean;
  /** For Storybook: pass links directly instead of fetching from API */
  initialLinks?: GitHubLink[];
  /** Force the card to show (toggle is on) */
  forceShow?: boolean;
  /** Callback when links are loaded/changed - used to sync toggle state */
  onHasLinksChange?: (hasLinks: boolean) => void;
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

function GitHubLinksCard({ sessionId, isOwner, initialLinks, forceShow, onHasLinksChange }: GitHubLinksCardProps) {
  const isVisible = useVisibility();

  // State
  const [links, setLinks] = useState<GitHubLink[]>(initialLinks ?? []);
  const [loading, setLoading] = useState(initialLinks === undefined);
  const [error, setError] = useState<string | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newUrl, setNewUrl] = useState('');
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);

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

  // Initial fetch with cleanup to prevent race conditions
  useEffect(() => {
    let cancelled = false;

    const doFetch = async () => {
      if (initialLinks !== undefined) return;
      try {
        setLoading(true);
        const response = await githubLinksAPI.list(sessionId);
        if (!cancelled) {
          setLinks(response.links);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          console.error('Failed to fetch GitHub links:', err);
          setError('Failed to load');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    doFetch();

    return () => {
      cancelled = true;
    };
  }, [sessionId, initialLinks]);

  // Poll when visible
  useEffect(() => {
    if (initialLinks !== undefined || !isVisible) return;
    const pollLinks = () => fetchLinks(false);
    const intervalId = setInterval(pollLinks, GITHUB_LINKS_POLL_INTERVAL_MS);
    return () => clearInterval(intervalId);
  }, [initialLinks, isVisible, fetchLinks]);

  // Auto-show add form when revealed via menu (only if empty)
  useEffect(() => {
    if (forceShow && links.length === 0 && !showAddForm && !loading) {
      setShowAddForm(true);
    }
  }, [forceShow, links.length, showAddForm, loading]);

  // Notify parent when links availability changes (for syncing toggle state)
  useEffect(() => {
    if (!loading && onHasLinksChange) {
      onHasLinksChange(links.length > 0);
    }
  }, [links.length, loading, onHasLinksChange]);

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

  const prLinks = useMemo(() => {
    return links
      .filter((link) => link.link_type === 'pull_request')
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  }, [links]);

  const formatRef = (link: GitHubLink) => {
    if (link.link_type === 'pull_request') {
      return `#${link.ref}`;
    }
    return link.ref.slice(0, 7);
  };

  // Don't render if:
  // - Non-owner with no links (nothing to show, can't add)
  // - Owner with forceShow=false (toggle is off)
  if (!loading && !isOwner && links.length === 0) {
    return null;
  }
  if (!loading && isOwner && !forceShow) {
    return null;
  }

  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <span className={styles.cardTitle}>
          <span className={styles.cardTitleIcon}>{GitHubIcon}</span>
          GitHub
        </span>
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
              placeholder="https://github.com/owner/repo/pull/123"
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
          <div className={styles.empty}>
            <span>No linked PRs or commits</span>
            {isOwner && (
              <button
                className={styles.emptyAddButton}
                onClick={() => setShowAddForm(true)}
              >
                Add link
              </button>
            )}
          </div>
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

            {/* Commit Links */}
            {commitLinks.map((link) => (
              <div key={link.id} className={styles.linkRow}>
                <a
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={styles.linkContent}
                  title={`${link.owner}/${link.repo}`}
                >
                  <span className={styles.linkIcon}>{CommitIcon}</span>
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
          </>
        )}
      </div>
    </div>
  );
}

export default GitHubLinksCard;
