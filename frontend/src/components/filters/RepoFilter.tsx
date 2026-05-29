import { useDropdown } from '@/hooks';
import { RepoIcon } from '@/components/icons';
import styles from '@/styles/filterDropdown.module.css';

export interface RepoFilterProps {
  /** Available repos to offer (owner/name form). */
  availableRepos: string[];
  /** Currently selected repos. Empty = all repos (CF-506 semantics). */
  selectedRepos: string[];
  /** Whether sessions without a repo are included. */
  includeNoRepo: boolean;
  /** Emits the next repo-filter slice; parent merges into its full value. */
  onChange: (next: { repos: string[]; includeNoRepo: boolean }) => void;
}

/**
 * Shared Repo dropdown for OrgFilters and TrendsFilters (CF-508). Owns the
 * button label, the Include-no-repo toggle, the per-repo checkbox list, and
 * the Clear button. Preserves the CF-233 / CF-506 contract: empty `repos[]`
 * means "all repos", there is no Select-all, and Clear leaves `includeNoRepo`
 * untouched.
 */
function RepoFilter({ availableRepos, selectedRepos, includeNoRepo, onChange }: RepoFilterProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Highlight the button when the selection is a strict subset of available,
  // or no-repo sessions are excluded.
  const isFiltered =
    (selectedRepos.length > 0 && selectedRepos.length < availableRepos.length) || !includeNoRepo;

  // CF-233 / CF-506: empty repos[] means "all repos". A subset selection shows
  // the count; selecting every chip is semantically the same as the empty
  // default, so it also reads "All Repos".
  function getLabel(): string {
    if (selectedRepos.length === 0 || selectedRepos.length === availableRepos.length) {
      return 'All Repos';
    }
    return `${selectedRepos.length} repo${selectedRepos.length > 1 ? 's' : ''}`;
  }

  const handleRepoToggle = (repo: string) => {
    const next = selectedRepos.includes(repo)
      ? selectedRepos.filter((r) => r !== repo)
      : [...selectedRepos, repo];
    onChange({ repos: next, includeNoRepo });
  };

  const handleIncludeNoRepoToggle = () => {
    onChange({ repos: selectedRepos, includeNoRepo: !includeNoRepo });
  };

  const handleClear = () => {
    onChange({ repos: [], includeNoRepo });
  };

  return (
    <div className={styles.filterWrapper} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${isFiltered ? styles.active : ''}`}
        onClick={toggle}
        title="Repository Filter"
        aria-label="Repository Filter"
        aria-expanded={isOpen}
      >
        {RepoIcon}
        <span className={styles.filterLabel}>{getLabel()}</span>
      </button>

      {isOpen && (
        <div className={styles.dropdown}>
          <div className={styles.dropdownContent}>
            <div className={styles.section}>
              <label className={styles.checkboxItem}>
                <input
                  type="checkbox"
                  checked={includeNoRepo}
                  onChange={handleIncludeNoRepoToggle}
                />
                <span>Include sessions without repo</span>
              </label>

              {availableRepos.length > 0 && (
                <>
                  <div className={styles.divider} />
                  <div className={styles.sectionHeader}>
                    <span className={styles.sectionLabel}>Filter by repo</span>
                    {selectedRepos.length > 0 && (
                      <button className={styles.clearBtn} onClick={handleClear}>
                        Clear
                      </button>
                    )}
                  </div>
                  {availableRepos.map((repo) => (
                    <label key={repo} className={styles.checkboxItem}>
                      <input
                        type="checkbox"
                        checked={selectedRepos.includes(repo)}
                        onChange={() => handleRepoToggle(repo)}
                      />
                      <span className={styles.repoName}>{repo}</span>
                    </label>
                  ))}
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default RepoFilter;
