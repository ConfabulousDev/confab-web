import { useState, useEffect, useRef, useCallback } from 'react';
import { useDropdown } from '@/hooks';
import { SearchIcon, RepoIcon, BranchIcon, UserIcon, CheckIcon, RobotIcon } from './icons';
import { getProviderIcon } from './providerIcon';
import type { SessionFilterOptions } from '@/schemas/api';
import { PROVIDER_VALUES, providerLabel } from '@/utils/providers';
import styles from './FilterChipsBar.module.css';

interface FilterChipsBarProps {
  filters: {
    repos: string[];
    branches: string[];
    owners: string[];
    providers: string[];
    query: string;
  };
  // Provider options are static and live on the frontend; only the
  // data-driven dimensions (repos/branches/owners) come from the backend.
  filterOptions: Pick<SessionFilterOptions, 'repos' | 'branches' | 'owners'> | null;
  currentUserEmail: string | null;
  onToggleRepo: (value: string) => void;
  onToggleBranch: (value: string) => void;
  onToggleOwner: (value: string) => void;
  onToggleProvider: (value: string) => void;
  onQueryChange: (value: string) => void;
  onClearAll: () => void;
  onCommitHistory?: () => void;
  // CF-393: Provider chip is opt-in. The session-listing endpoint supports
  // ?provider= filtering; the TILs endpoint does not, so TILsPage omits it.
  showProviderFilter?: boolean;
}

interface DimensionDropdownProps {
  label: string;
  icon: React.ReactNode;
  options: string[];
  selected: string[];
  currentUserEmail?: string | null;
  onToggle: (value: string) => void;
  // Optional per-option icon and display-label overrides. When omitted,
  // rows render exactly as before (no icon column, raw option as label).
  iconFor?: (option: string) => React.ReactNode;
  labelFor?: (option: string) => string;
}

function DimensionDropdown({
  label,
  icon,
  options,
  selected,
  currentUserEmail,
  onToggle,
  iconFor,
  labelFor,
}: DimensionDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();
  const [search, setSearch] = useState('');

  const handleToggle = () => {
    if (!isOpen) setSearch('');
    toggle();
  };

  const filtered = search
    ? options.filter((o) => o.toLowerCase().includes(search.toLowerCase()))
    : options;

  // Sort: selected first, then alphabetical by display label
  const sorted = [...filtered].sort((a, b) => {
    const aSelected = selected.includes(a) ? 0 : 1;
    const bSelected = selected.includes(b) ? 0 : 1;
    if (aSelected !== bSelected) return aSelected - bSelected;
    const aLabel = labelFor ? labelFor(a) : a;
    const bLabel = labelFor ? labelFor(b) : b;
    return aLabel.localeCompare(bLabel);
  });

  return (
    <div className={styles.dimensionContainer} ref={containerRef}>
      <button
        className={`${styles.dimensionBtn} ${selected.length > 0 ? styles.dimensionActive : ''}`}
        onClick={handleToggle}
        aria-expanded={isOpen}
      >
        <span className={styles.dimensionIcon}>{icon}</span>
        {label}
        {selected.length > 0 && <span className={styles.dimensionBadge}>{selected.length}</span>}
      </button>
      {isOpen && (
        <div className={styles.dimensionDropdown}>
          {options.length > 5 && (
            <div className={styles.dimensionSearch}>
              <input
                type="text"
                placeholder={`Search ${label.toLowerCase()}...`}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className={styles.dimensionSearchInput}
                autoFocus
              />
            </div>
          )}
          <div className={styles.dimensionList}>
            {sorted.map((opt) => {
              const isSelected = selected.includes(opt);
              const baseLabel = labelFor ? labelFor(opt) : opt;
              const displayLabel = currentUserEmail && opt.toLowerCase() === currentUserEmail.toLowerCase()
                ? `${baseLabel} (you)`
                : baseLabel;
              return (
                <button
                  key={opt}
                  className={`${styles.dimensionItem} ${isSelected ? styles.dimensionItemSelected : ''}`}
                  onClick={() => onToggle(opt)}
                >
                  <span className={`${styles.checkbox} ${isSelected ? styles.checked : ''}`}>
                    {CheckIcon}
                  </span>
                  {iconFor && <span className={styles.dimensionIcon}>{iconFor(opt)}</span>}
                  <span className={styles.dimensionLabel}>{displayLabel}</span>
                </button>
              );
            })}
            {sorted.length === 0 && (
              <div className={styles.dimensionEmpty}>No matches</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function FilterChipsBar({
  filters,
  filterOptions,
  currentUserEmail,
  onToggleRepo,
  onToggleBranch,
  onToggleOwner,
  onToggleProvider,
  onQueryChange,
  onClearAll,
  onCommitHistory,
  showProviderFilter = true,
}: FilterChipsBarProps) {
  // Debounce search: keep local state responsive, defer URL/API update
  const [localQuery, setLocalQuery] = useState(filters.query);
  const [prevQuery, setPrevQuery] = useState(filters.query);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Sync local state when external filters change (e.g. clear all, back/forward nav)
  // Uses the React-recommended "adjusting state during rendering" pattern
  if (filters.query !== prevQuery) {
    setPrevQuery(filters.query);
    setLocalQuery(filters.query);
  }

  const handleQueryChange = useCallback(
    (value: string) => {
      setLocalQuery(value);
      clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => onQueryChange(value), 300);
    },
    [onQueryChange]
  );

  // Cleanup timer on unmount
  useEffect(() => () => clearTimeout(timerRef.current), []);

  const showProviderActive = showProviderFilter && filters.providers.length > 0;
  const hasActiveFilters =
    filters.repos.length > 0 ||
    filters.branches.length > 0 ||
    filters.owners.length > 0 ||
    showProviderActive ||
    filters.query !== '';

  return (
    <div className={styles.container}>
      <div className={styles.controlsRow}>
        <div className={styles.searchWrapper}>
          <span className={styles.searchIcon}>{SearchIcon}</span>
          <input
            type="text"
            className={styles.searchInput}
            placeholder="Search sessions..."
            value={localQuery}
            onChange={(e) => handleQueryChange(e.target.value)}
            onBlur={onCommitHistory}
          />
        </div>
        <div className={styles.dimensionButtons}>
          {/* CF-393: Provider chip first — coarsest cut. */}
          {showProviderFilter && (
            <DimensionDropdown
              label="Provider"
              icon={RobotIcon}
              options={[...PROVIDER_VALUES]}
              selected={filters.providers}
              onToggle={onToggleProvider}
              iconFor={getProviderIcon}
              labelFor={providerLabel}
            />
          )}
          {filterOptions && filterOptions.repos.length > 0 && (
            <DimensionDropdown
              label="Repo"
              icon={RepoIcon}
              options={filterOptions.repos}
              selected={filters.repos}
              onToggle={onToggleRepo}
            />
          )}
          {filterOptions && filterOptions.branches.length > 0 && (
            <DimensionDropdown
              label="Branch"
              icon={BranchIcon}
              options={filterOptions.branches}
              selected={filters.branches}
              onToggle={onToggleBranch}
            />
          )}
          {filterOptions && filterOptions.owners.length > 0 && (
            <DimensionDropdown
              label="Owner"
              icon={UserIcon}
              options={filterOptions.owners}
              selected={filters.owners}
              currentUserEmail={currentUserEmail}
              onToggle={onToggleOwner}
            />
          )}
        </div>
      </div>

      {hasActiveFilters && (
        <div className={styles.chipsRow}>
          {showProviderActive && filters.providers.map((p) => (
            <button
              key={`provider:${p}`}
              className={styles.chip}
              onClick={() => onToggleProvider(p)}
              aria-label={`provider: ${providerLabel(p)}`}
            >
              <span className={styles.dimensionIcon}>{getProviderIcon(p)}</span>
              <span className={styles.chipDimension}>provider:</span> {providerLabel(p)}{' '}
              <span className={styles.chipRemove}>&times;</span>
            </button>
          ))}
          {filters.repos.map((repo) => (
            <button key={`repo:${repo}`} className={styles.chip} onClick={() => onToggleRepo(repo)}>
              <span className={styles.chipDimension}>repo:</span> {repo} <span className={styles.chipRemove}>&times;</span>
            </button>
          ))}
          {filters.branches.map((branch) => (
            <button key={`branch:${branch}`} className={styles.chip} onClick={() => onToggleBranch(branch)}>
              <span className={styles.chipDimension}>branch:</span> {branch} <span className={styles.chipRemove}>&times;</span>
            </button>
          ))}
          {filters.owners.map((owner) => (
            <button key={`owner:${owner}`} className={styles.chip} onClick={() => onToggleOwner(owner)}>
              <span className={styles.chipDimension}>owner:</span> {owner} <span className={styles.chipRemove}>&times;</span>
            </button>
          ))}
          <button className={styles.clearBtn} onClick={onClearAll}>
            Clear all
          </button>
        </div>
      )}
    </div>
  );
}

export default FilterChipsBar;
