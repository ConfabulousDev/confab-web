// OpenCode transcript filter dropdown (MVP). Parallel to CodexFilterDropdown
// but with a flat three-category model (User / Assistant / Tool) — no
// subcategories. Shares FilterDropdownShared.module.css for visual parity.
//
// Zero-count chips render disabled+greyed; the toggle hides/shows that
// category's rows in the transcript pane.

import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon } from '../icons';
import type {
  OpenCodeCategory,
  OpenCodeFilterState,
  OpenCodeHierarchicalCounts,
} from './opencodeCategories';
import styles from './FilterDropdownShared.module.css';

interface OpenCodeFilterDropdownProps {
  counts: OpenCodeHierarchicalCounts;
  filterState: OpenCodeFilterState;
  onToggleCategory: (category: OpenCodeCategory) => void;
}

const CATEGORIES: Array<{ category: OpenCodeCategory; label: string; color: string }> = [
  { category: 'user', label: 'User', color: '#16a34a' },
  { category: 'assistant', label: 'Assistant', color: '#2563eb' },
  { category: 'tool', label: 'Tool Call', color: '#d97706' },
];

export default function OpenCodeFilterDropdown({
  counts,
  filterState,
  onToggleCategory,
}: OpenCodeFilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  const hasActiveFilters = CATEGORIES.some(
    ({ category }) => counts[category] > 0 && !filterState[category],
  );

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${hasActiveFilters ? styles.active : ''}`}
        onClick={toggle}
        title="Message Filters"
        aria-label="Message Filters"
        aria-expanded={isOpen}
      >
        {FilterIcon}
      </button>

      {isOpen && (
        <div className={styles.dropdown}>
          <div className={styles.dropdownHeader}>Message Filters</div>
          <div className={styles.dropdownContent}>
            {CATEGORIES.map(({ category, label, color }) => {
              const count = counts[category];
              const isVisible = filterState[category];
              const isDisabled = count === 0;
              return (
                <button
                  key={category}
                  className={`${styles.filterItem} ${styles.flatItem} ${isDisabled ? styles.disabled : ''}`}
                  onClick={() => !isDisabled && onToggleCategory(category)}
                  disabled={isDisabled}
                >
                  <span
                    className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                    style={{ color: isVisible ? color : undefined }}
                  >
                    {CheckIcon}
                  </span>
                  <span className={styles.filterLabel}>{label}</span>
                  <span className={styles.filterCount}>{count}</span>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
