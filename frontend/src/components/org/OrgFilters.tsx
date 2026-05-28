import { useMemo } from 'react';
import { useDropdown } from '@/hooks';
import type { DateRange } from '@/utils/dateRange';
import { getDatePresets } from '@/utils/dateRange';
import { CalendarIcon, CheckIcon, RepoIcon, RobotIcon } from '@/components/icons';
import { getProviderIcon } from '@/components/providerIcon';
import { providerLabel } from '@/utils/providers';
import styles from './OrgFilters.module.css';

export interface OrgFiltersValue {
  dateRange: DateRange;
  // Canonical providers (`claude-code`, `codex`). Empty = aggregate across all
  // providers — same wire semantics as the trends filter.
  providers: string[];
  // Repo names (owner/name form) to include. Empty = include every repo
  // (CF-506 semantics, matching /sessions). `includeNoRepo` independently
  // controls whether sessions without a repo count.
  repos: string[];
  includeNoRepo: boolean;
}

interface OrgFiltersProps {
  /**
   * Canonical providers to offer in the dropdown. The page narrows this to
   * `providers_present` after the first response lands so empty providers
   * don't appear; before then it passes the full canonical list.
   */
  availableProviders: string[];
  /** Org-wide repos in the current date range (from `/org/repos`). */
  availableRepos: string[];
  value: OrgFiltersValue;
  onChange: (value: OrgFiltersValue) => void;
}

function OrgFilters({ availableProviders, availableRepos, value, onChange }: OrgFiltersProps) {
  const {
    isOpen: providerIsOpen,
    toggle: toggleProvider,
    containerRef: providerContainerRef,
  } = useDropdown<HTMLDivElement>();
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

  // Highlight the repo button when the selection is a strict subset of
  // available, or no-repo sessions are excluded (mirrors TrendsFilters).
  const isRepoFiltered =
    (value.repos.length > 0 && value.repos.length < availableRepos.length) || !value.includeNoRepo;

  const handleDateRangeChange = (preset: DateRange) => {
    onChange({ ...value, dateRange: preset });
    setDateIsOpen(false);
  };

  const handleProviderToggle = (provider: string) => {
    const next = value.providers.includes(provider)
      ? value.providers.filter((p) => p !== provider)
      : [...value.providers, provider];
    onChange({ ...value, providers: next });
  };

  const handleRepoToggle = (repo: string) => {
    const next = value.repos.includes(repo)
      ? value.repos.filter((r) => r !== repo)
      : [...value.repos, repo];
    onChange({ ...value, repos: next });
  };

  const handleIncludeNoRepoToggle = () => {
    onChange({ ...value, includeNoRepo: !value.includeNoRepo });
  };

  const handleClearRepos = () => {
    onChange({ ...value, repos: [] });
  };

  function getProviderButtonLabel(): string {
    if (value.providers.length === 0) return 'All Providers';
    if (value.providers.length === 1) return providerLabel(value.providers[0] ?? '');
    return `${value.providers.length} providers`;
  }

  // CF-233 / CF-506: empty repos[] means "all repos". A subset selection
  // shows the count; selecting every chip is semantically the same as the
  // empty default, so it also reads "All Repos".
  function getRepoLabel(): string {
    if (value.repos.length === 0 || value.repos.length === availableRepos.length) {
      return 'All Repos';
    }
    const count = value.repos.length;
    return `${count} repo${count > 1 ? 's' : ''}`;
  }

  return (
    <div className={styles.container}>
      {/* Provider Filter */}
      <div className={styles.filterWrapper} ref={providerContainerRef}>
        <button
          className={`${styles.filterBtn} ${value.providers.length > 0 ? styles.active : ''}`}
          onClick={toggleProvider}
          title="Provider Filter"
          aria-label="Provider Filter"
          aria-expanded={providerIsOpen}
        >
          {RobotIcon}
          <span className={styles.filterLabel}>{getProviderButtonLabel()}</span>
        </button>

        {providerIsOpen && (
          <div className={styles.dropdown}>
            <div className={styles.dropdownContent}>
              <div className={styles.section}>
                {availableProviders.map((p) => (
                  <label key={p} className={styles.checkboxItem}>
                    <input
                      type="checkbox"
                      checked={value.providers.includes(p)}
                      onChange={() => handleProviderToggle(p)}
                    />
                    <span className={styles.providerIcon}>{getProviderIcon(p)}</span>
                    <span>{providerLabel(p)}</span>
                  </label>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>

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
          className={`${styles.filterBtn} ${isRepoFiltered ? styles.active : ''}`}
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

                {availableRepos.length > 0 && (
                  <>
                    <div className={styles.divider} />
                    <div className={styles.sectionHeader}>
                      <span className={styles.sectionLabel}>Filter by repo</span>
                      {value.repos.length > 0 && (
                        <button className={styles.toggleAllBtn} onClick={handleClearRepos}>
                          Clear
                        </button>
                      )}
                    </div>
                    {availableRepos.map((repo) => (
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

export default OrgFilters;
