import { useState } from 'react';
import { useDropdown } from '@/hooks';
import { SearchIcon, RepoIcon, BranchIcon, UserIcon, CheckIcon } from './icons';
import type { SessionFilterOptions } from '@/schemas/api';
import styles from './FilterChipsBar.module.css';

interface FilterChipsBarProps {
  filters: {
    repos: string[];
    branches: string[];
    owners: string[];
    query: string;
  };
  filterOptions: SessionFilterOptions | null;
  currentUserEmail: string | null;
  onToggleRepo: (value: string) => void;
  onToggleBranch: (value: string) => void;
  onToggleOwner: (value: string) => void;
  onQueryChange: (value: string) => void;
  onClearAll: () => void;
}

interface DimensionDropdownProps {
  label: string;
  icon: React.ReactNode;
  options: { value: string; count: number }[];
  selected: string[];
  currentUserEmail?: string | null;
  onToggle: (value: string) => void;
}

function DimensionDropdown({ label, icon, options, selected, currentUserEmail, onToggle }: DimensionDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();
  const [search, setSearch] = useState('');

  const handleToggle = () => {
    if (!isOpen) setSearch('');
    toggle();
  };

  const filtered = search
    ? options.filter((o) => o.value.toLowerCase().includes(search.toLowerCase()))
    : options;

  // Sort: selected first, then by count desc
  const sorted = [...filtered].sort((a, b) => {
    const aSelected = selected.includes(a.value) ? 0 : 1;
    const bSelected = selected.includes(b.value) ? 0 : 1;
    if (aSelected !== bSelected) return aSelected - bSelected;
    return b.count - a.count;
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
              const isSelected = selected.includes(opt.value);
              const displayLabel = currentUserEmail && opt.value.toLowerCase() === currentUserEmail.toLowerCase()
                ? `${opt.value} (you)`
                : opt.value;
              return (
                <button
                  key={opt.value}
                  className={`${styles.dimensionItem} ${isSelected ? styles.dimensionItemSelected : ''}`}
                  onClick={() => onToggle(opt.value)}
                >
                  <span className={`${styles.checkbox} ${isSelected ? styles.checked : ''}`}>
                    {CheckIcon}
                  </span>
                  <span className={styles.dimensionLabel}>{displayLabel}</span>
                  <span className={styles.dimensionCount}>{opt.count}</span>
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
  onQueryChange,
  onClearAll,
}: FilterChipsBarProps) {
  const hasActiveFilters =
    filters.repos.length > 0 ||
    filters.branches.length > 0 ||
    filters.owners.length > 0 ||
    filters.query !== '';

  return (
    <div className={styles.container}>
      <div className={styles.controlsRow}>
        <div className={styles.searchWrapper}>
          <span className={styles.searchIcon}>{SearchIcon}</span>
          <input
            type="text"
            className={styles.searchInput}
            placeholder="Search by title or commit..."
            value={filters.query}
            onChange={(e) => onQueryChange(e.target.value)}
          />
        </div>
        <div className={styles.dimensionButtons}>
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
