import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon, GitHubIcon, BranchIcon, ComputerIcon, SearchIcon, PRIcon, CommitIcon } from './icons';
import styles from './SessionsFilterDropdown.module.css';

interface SessionsFilterDropdownProps {
  repos: string[];
  branches: string[];
  hostnames: string[];
  prs: string[];
  selectedRepo: string | null;
  selectedBranch: string | null;
  selectedHostname: string | null;
  selectedPR: string | null;
  commitSearch: string;
  repoCounts: Record<string, number>;
  branchCounts: Record<string, number>;
  hostnameCounts: Record<string, number>;
  prCounts: Record<string, number>;
  totalCount: number;
  searchQuery: string;
  onRepoClick: (repo: string | null) => void;
  onBranchClick: (branch: string | null) => void;
  onHostnameClick: (hostname: string | null) => void;
  onPRClick: (pr: string | null) => void;
  onCommitSearchChange: (commit: string) => void;
  onSearchChange: (query: string) => void;
}

function SessionsFilterDropdown({
  repos,
  branches,
  hostnames,
  prs,
  selectedRepo,
  selectedBranch,
  selectedHostname,
  selectedPR,
  commitSearch,
  repoCounts,
  branchCounts,
  hostnameCounts,
  prCounts,
  totalCount,
  searchQuery,
  onRepoClick,
  onBranchClick,
  onHostnameClick,
  onPRClick,
  onCommitSearchChange,
  onSearchChange,
}: SessionsFilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Check if any filters are active
  const hasActiveFilters = selectedRepo !== null || selectedBranch !== null || selectedHostname !== null || selectedPR !== null || commitSearch !== '' || searchQuery !== '';

  // Get visible branches (only show branches with sessions when a repo is selected)
  const visibleBranches = selectedRepo
    ? branches.filter((branch) => (branchCounts[branch] ?? 0) > 0)
    : [];

  // Get visible PRs (only show when a repo is selected and PRs have matching sessions)
  const visiblePRs = selectedRepo
    ? prs.filter((pr) => (prCounts[pr] ?? 0) > 0)
    : [];

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${hasActiveFilters ? styles.active : ''}`}
        onClick={toggle}
        title="Session Filters"
        aria-label="Session Filters"
        aria-expanded={isOpen}
      >
        {FilterIcon}
      </button>

      {isOpen && (
        <div className={styles.dropdown}>
          <div className={styles.dropdownHeader}>Session Filters</div>
          <div className={styles.dropdownContent}>
            {/* Search input */}
            <div className={styles.searchWrapper}>
              <span className={styles.searchIcon}>{SearchIcon}</span>
              <input
                type="text"
                className={styles.searchInput}
                placeholder="Search by title..."
                value={searchQuery}
                onChange={(e) => onSearchChange(e.target.value)}
                onClick={(e) => e.stopPropagation()}
              />
            </div>

            {/* All Sessions option */}
            <button
              className={`${styles.filterItem} ${!selectedRepo && !selectedHostname && !selectedPR && !commitSearch ? styles.selected : ''}`}
              onClick={() => {
                onRepoClick(null);
                onHostnameClick(null);
                onPRClick(null);
                onCommitSearchChange('');
              }}
            >
              <span className={`${styles.checkbox} ${!selectedRepo && !selectedHostname && !selectedPR && !commitSearch ? styles.checked : ''}`}>
                {CheckIcon}
              </span>
              <span className={styles.filterLabel}>All Sessions</span>
              <span className={styles.filterCount}>{totalCount}</span>
            </button>

            {/* Repos section */}
            {repos.length > 0 && (
              <>
                <div className={styles.sectionDivider} />
                <div className={styles.sectionLabel}>Repositories</div>
                {repos.map((repo) => (
                  <button
                    key={repo}
                    className={`${styles.filterItem} ${selectedRepo === repo ? styles.selected : ''}`}
                    onClick={() => onRepoClick(selectedRepo === repo ? null : repo)}
                  >
                    <span className={`${styles.checkbox} ${selectedRepo === repo ? styles.checked : ''}`}>
                      {CheckIcon}
                    </span>
                    <span className={styles.iconWrapper}>{GitHubIcon}</span>
                    <span className={styles.filterLabel}>{repo}</span>
                    <span className={styles.filterCount}>{repoCounts[repo] || 0}</span>
                  </button>
                ))}
              </>
            )}

            {/* Branches section - only show when a repo is selected */}
            {visibleBranches.length > 0 && (
              <>
                <div className={styles.sectionDivider} />
                <div className={styles.sectionLabel}>Branches</div>
                {visibleBranches.map((branch) => (
                  <button
                    key={branch}
                    className={`${styles.filterItem} ${selectedBranch === branch ? styles.selected : ''}`}
                    onClick={() => onBranchClick(selectedBranch === branch ? null : branch)}
                  >
                    <span className={`${styles.checkbox} ${selectedBranch === branch ? styles.checked : ''}`}>
                      {CheckIcon}
                    </span>
                    <span className={styles.iconWrapper}>{BranchIcon}</span>
                    <span className={styles.filterLabel}>{branch}</span>
                    <span className={styles.filterCount}>{branchCounts[branch] || 0}</span>
                  </button>
                ))}
              </>
            )}

            {/* PRs section - only show when a repo is selected */}
            {visiblePRs.length > 0 && (
              <>
                <div className={styles.sectionDivider} />
                <div className={styles.sectionLabel}>Pull Requests</div>
                {visiblePRs.map((pr) => (
                  <button
                    key={pr}
                    className={`${styles.filterItem} ${selectedPR === pr ? styles.selected : ''}`}
                    onClick={() => onPRClick(selectedPR === pr ? null : pr)}
                  >
                    <span className={`${styles.checkbox} ${selectedPR === pr ? styles.checked : ''}`}>
                      {CheckIcon}
                    </span>
                    <span className={styles.iconWrapper}>{PRIcon}</span>
                    <span className={styles.filterLabel}>#{pr}</span>
                    <span className={styles.filterCount}>{prCounts[pr] || 0}</span>
                  </button>
                ))}
              </>
            )}

            {/* Commit search - only show when a repo is selected */}
            {selectedRepo && (
              <>
                <div className={styles.sectionDivider} />
                <div className={styles.sectionLabel}>Commit</div>
                <div className={styles.searchWrapper}>
                  <span className={styles.searchIcon}>{CommitIcon}</span>
                  <input
                    type="text"
                    className={styles.searchInput}
                    placeholder="Filter by commit SHA..."
                    value={commitSearch}
                    onChange={(e) => onCommitSearchChange(e.target.value)}
                    onClick={(e) => e.stopPropagation()}
                  />
                </div>
              </>
            )}

            {/* Hostnames section */}
            {hostnames.length > 0 && (
              <>
                <div className={styles.sectionDivider} />
                <div className={styles.sectionLabel}>Hostnames</div>
                {hostnames.map((hostname) => (
                  <button
                    key={hostname}
                    className={`${styles.filterItem} ${selectedHostname === hostname ? styles.selected : ''}`}
                    onClick={() => onHostnameClick(selectedHostname === hostname ? null : hostname)}
                  >
                    <span className={`${styles.checkbox} ${selectedHostname === hostname ? styles.checked : ''}`}>
                      {CheckIcon}
                    </span>
                    <span className={styles.iconWrapper}>{ComputerIcon}</span>
                    <span className={styles.filterLabel}>{hostname}</span>
                    <span className={styles.filterCount}>{hostnameCounts[hostname] || 0}</span>
                  </button>
                ))}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default SessionsFilterDropdown;
