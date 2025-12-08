import { useState, useRef, useEffect } from 'react';
import styles from './SessionsFilterDropdown.module.css';

// SVG Icons
const FilterIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
  </svg>
);

const CheckIcon = (
  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
    <polyline points="20 6 9 17 4 12" />
  </svg>
);

const GitHubIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
  </svg>
);

const BranchIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="6" y1="3" x2="6" y2="15" />
    <circle cx="18" cy="6" r="3" />
    <circle cx="6" cy="18" r="3" />
    <path d="M18 9a9 9 0 0 1-9 9" />
  </svg>
);

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
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Check if any filters are active
  const hasActiveFilters = selectedRepo !== null || selectedBranch !== null;

  // Close dropdown when clicking outside
  useEffect(() => {
    if (!isOpen) return;

    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        event.target instanceof Node &&
        !containerRef.current.contains(event.target)
      ) {
        setIsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen]);

  // Close on escape key
  useEffect(() => {
    if (!isOpen) return;

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    }

    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen]);

  const handleRepoClick = (repo: string | null) => {
    onRepoClick(repo);
  };

  const handleBranchClick = (branch: string | null) => {
    onBranchClick(branch);
  };

  // Get visible branches (only show branches with sessions when a repo is selected)
  const visibleBranches = selectedRepo
    ? branches.filter((branch) => (branchCounts[branch] ?? 0) > 0)
    : [];

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${hasActiveFilters ? styles.active : ''}`}
        onClick={() => setIsOpen(!isOpen)}
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
              onClick={() => handleRepoClick(null)}
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
                    onClick={() => handleRepoClick(selectedRepo === repo ? null : repo)}
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
                    onClick={() => handleBranchClick(selectedBranch === branch ? null : branch)}
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
