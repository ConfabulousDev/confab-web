import { useMemo } from 'react';
import { useDropdown } from '@/hooks';
import type { DateRange } from '@/utils/dateRange';
import { getDatePresets } from '@/utils/dateRange';
import { CalendarIcon, CheckIcon } from '@/components/icons';
import ProviderFilter from '@/components/filters/ProviderFilter';
import RepoFilter from '@/components/filters/RepoFilter';
import styles from '@/styles/filterDropdown.module.css';

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
    isOpen: dateIsOpen,
    setIsOpen: setDateIsOpen,
    toggle: toggleDate,
    containerRef: dateContainerRef,
  } = useDropdown<HTMLDivElement>();

  const datePresets = useMemo(() => getDatePresets(), []);

  const handleDateRangeChange = (preset: DateRange) => {
    onChange({ ...value, dateRange: preset });
    setDateIsOpen(false);
  };

  return (
    <div className={styles.container}>
      {/* Provider Filter */}
      <ProviderFilter
        availableProviders={availableProviders}
        selectedProviders={value.providers}
        onChange={(providers) => onChange({ ...value, providers })}
      />

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
      <RepoFilter
        availableRepos={availableRepos}
        selectedRepos={value.repos}
        includeNoRepo={value.includeNoRepo}
        onChange={(next) => onChange({ ...value, ...next })}
      />
    </div>
  );
}

export default OrgFilters;
