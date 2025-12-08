import { useState, useRef, useEffect } from 'react';
import type { MessageCategory, MessageCategoryCounts } from './messageCategories';
import type { SidebarItemColor } from '../PageSidebar';
import styles from './FilterDropdown.module.css';

interface FilterDropdownProps {
  counts: MessageCategoryCounts;
  visibleCategories: Set<MessageCategory>;
  onToggleCategory: (category: MessageCategory) => void;
}

interface FilterItemConfig {
  category: MessageCategory;
  label: string;
  color: SidebarItemColor;
}

const FILTER_ITEMS: FilterItemConfig[] = [
  { category: 'user', label: 'User', color: 'green' },
  { category: 'assistant', label: 'Assistant', color: 'blue' },
  { category: 'system', label: 'System', color: 'gray' },
  { category: 'file-history-snapshot', label: 'File Snapshot', color: 'cyan' },
  { category: 'summary', label: 'Summary', color: 'purple' },
  { category: 'queue-operation', label: 'Queue', color: 'amber' },
];

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

function FilterDropdown({ counts, visibleCategories, onToggleCategory }: FilterDropdownProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Check if any filters are active (not showing all categories)
  const totalCategories = FILTER_ITEMS.filter((item) => counts[item.category] > 0).length;
  const activeFilters = FILTER_ITEMS.filter(
    (item) => counts[item.category] > 0 && visibleCategories.has(item.category)
  ).length;
  const hasActiveFilters = activeFilters !== totalCategories;

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

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${hasActiveFilters ? styles.active : ''}`}
        onClick={() => setIsOpen(!isOpen)}
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
            {FILTER_ITEMS.map((item) => {
              const count = counts[item.category];
              const isVisible = visibleCategories.has(item.category);
              const isDisabled = count === 0;

              return (
                <button
                  key={item.category}
                  className={`${styles.filterItem} ${isDisabled ? styles.disabled : ''}`}
                  onClick={() => !isDisabled && onToggleCategory(item.category)}
                  disabled={isDisabled}
                >
                  <span
                    className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                    style={{ color: isVisible ? getColorValue(item.color) : undefined }}
                  >
                    {CheckIcon}
                  </span>
                  <span className={`${styles.colorDot} ${styles[item.color]}`} />
                  <span className={styles.filterLabel}>{item.label}</span>
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

// Helper to get actual color value for checkbox
function getColorValue(color: SidebarItemColor): string {
  const colors: Record<SidebarItemColor, string> = {
    default: '#2563eb',
    green: '#16a34a',
    blue: '#2563eb',
    gray: '#6b7280',
    cyan: '#0284c7',
    purple: '#7c3aed',
    amber: '#d97706',
  };
  return colors[color];
}

export default FilterDropdown;
