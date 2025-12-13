import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon, GitHubIcon, BranchIcon } from './icons';
import styles from './SessionsFilterDropdown.module.css';

interface SessionsFilterDropdownProps {
  repos: string[];
  branches: string[];
  selectedRepo: string | null;
  selectedBranch: string | null;
  repoCounts: Record<string, number>;
  branchCounts: Record<string, number>;
  totalCount: number;
  onRepoClick: (repo: string | null) => void;
  onBranchClick: (branch: string | null) => void;
}

function SessionsFilterDropdown({
  repos,
  branches,
  selectedRepo,
  selectedBranch,
  repoCounts,
  branchCounts,
  totalCount,
  onRepoClick,
  onBranchClick,
}: SessionsFilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Check if any filters are active
  const hasActiveFilters = selectedRepo !== null || selectedBranch !== null;

  // Get visible branches (only show branches with sessions when a repo is selected)
  const visibleBranches = selectedRepo
    ? branches.filter((branch) => (branchCounts[branch] ?? 0) > 0)
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
            {/* All Sessions option */}
            <button
              className={`${styles.filterItem} ${!selectedRepo ? styles.selected : ''}`}
              onClick={() => onRepoClick(null)}
            >
              <span className={`${styles.checkbox} ${!selectedRepo ? styles.checked : ''}`}>
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
          </div>
        </div>
      )}
    </div>
  );
}

export default SessionsFilterDropdown;
