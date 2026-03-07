import { useMemo } from 'react';
import { useDropdown } from '@/hooks';
import type { DateRange } from '@/utils/dateRange';
import { getDatePresets } from '@/utils/dateRange';
import { CalendarIcon, RepoIcon, CheckIcon } from '@/components/icons';
import styles from './TrendsFilters.module.css';

export interface TrendsFiltersValue {
  dateRange: DateRange;
  repos: string[];
  includeNoRepo: boolean;
}

interface TrendsFiltersProps {
  repos: string[];
  value: TrendsFiltersValue;
  onChange: (value: TrendsFiltersValue) => void;
}

function TrendsFilters({ repos, value, onChange }: TrendsFiltersProps) {
  const {
    isOpen: dateIsOpen,
    setIsOpen: setDateIsOpen,
    toggle: toggleDate,
    containerRef: dateContainerRef,
  } = useDropdown<HTMLDivElement>();
  const {
    isOpen: repoIsOpen,
    toggle: toggleRepo,
    containerRef: repoContainerRef,
  } = useDropdown<HTMLDivElement>();

  const datePresets = useMemo(() => getDatePresets(), []);

  // Determine if we're showing a filtered subset
  const isFiltered = (value.repos.length > 0 && value.repos.length < repos.length) || !value.includeNoRepo;

  const handleDateRangeChange = (preset: DateRange) => {
    onChange({ ...value, dateRange: preset });
    setDateIsOpen(false);
  };

  const handleRepoToggle = (repo: string) => {
    const newRepos = value.repos.includes(repo)
      ? value.repos.filter((r) => r !== repo)
      : [...value.repos, repo];
    onChange({ ...value, repos: newRepos });
  };

  const handleIncludeNoRepoToggle = () => {
    onChange({ ...value, includeNoRepo: !value.includeNoRepo });
  };

  const handleSelectAllRepos = () => {
    onChange({ ...value, repos: [...repos] });
  };

  const handleDeselectAllRepos = () => {
    onChange({ ...value, repos: [] });
  };

  const allReposSelected = repos.length > 0 && value.repos.length === repos.length;

  function getRepoLabel(): string {
    if (allReposSelected) return 'All Repos';
    if (value.repos.length === 0) return 'No Repos';
    const count = value.repos.length;
    return `${count} repo${count > 1 ? 's' : ''}`;
  }

  return (
    <div className={styles.container}>
      {/* Date Range Filter */}
      <div className={styles.filterWrapper} ref={dateContainerRef}>
        <button
          className={styles.filterBtn}
          onClick={toggleDate}
          title="Date Range"
          aria-label="Date Range"
          aria-expanded={dateIsOpen}
        >
          {CalendarIcon}
          <span className={styles.filterLabel}>{value.dateRange.label}</span>
        </button>

        {dateIsOpen && (
          <div className={styles.dropdown}>
            <div className={styles.dropdownContent}>
              <div className={styles.section}>
                {datePresets.map((preset) => (
                  <button
                    key={preset.label}
                    className={`${styles.filterItem} ${value.dateRange.label === preset.label ? styles.selected : ''}`}
                    onClick={() => handleDateRangeChange(preset)}
                  >
                    <span className={styles.itemLabel}>{preset.label}</span>
                    {value.dateRange.label === preset.label && (
                      <span className={styles.checkIcon}>{CheckIcon}</span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Repo Filter */}
      <div className={styles.filterWrapper} ref={repoContainerRef}>
        <button
          className={`${styles.filterBtn} ${isFiltered ? styles.active : ''}`}
          onClick={toggleRepo}
          title="Repository Filter"
          aria-label="Repository Filter"
          aria-expanded={repoIsOpen}
        >
          {RepoIcon}
          <span className={styles.filterLabel}>{getRepoLabel()}</span>
        </button>

        {repoIsOpen && (
          <div className={styles.dropdown}>
            <div className={styles.dropdownContent}>
              <div className={styles.section}>
                <label className={styles.checkboxItem}>
                  <input
                    type="checkbox"
                    checked={value.includeNoRepo}
                    onChange={handleIncludeNoRepoToggle}
                  />
                  <span>Include sessions without repo</span>
                </label>

                {repos.length > 0 && (
                  <>
                    <div className={styles.divider} />
                    <div className={styles.sectionHeader}>
                      <span className={styles.sectionLabel}>Filter by repo</span>
                      <button
                        className={styles.toggleAllBtn}
                        onClick={allReposSelected ? handleDeselectAllRepos : handleSelectAllRepos}
                      >
                        {allReposSelected ? 'Deselect all' : 'Select all'}
                      </button>
                    </div>
                    {repos.map((repo) => (
                      <label key={repo} className={styles.checkboxItem}>
                        <input
                          type="checkbox"
                          checked={value.repos.includes(repo)}
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
    </div>
  );
}

export default TrendsFilters;
