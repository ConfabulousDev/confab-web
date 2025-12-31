import { useState } from 'react';
import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon } from '../icons';
import type {
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  HierarchicalCounts,
  FilterState,
} from './messageCategories';
import type { SidebarItemColor } from '../PageSidebar';
import styles from './FilterDropdown.module.css';

interface FilterDropdownProps {
  counts: HierarchicalCounts;
  filterState: FilterState;
  onToggleCategory: (category: MessageCategory) => void;
  onToggleUserSubcategory: (subcategory: UserSubcategory) => void;
  onToggleAssistantSubcategory: (subcategory: AssistantSubcategory) => void;
}

// Subcategory configurations
const USER_SUBCATEGORIES: Array<{ key: UserSubcategory; label: string }> = [
  { key: 'prompt', label: 'Prompts' },
  { key: 'tool-result', label: 'Tool Results' },
];

const ASSISTANT_SUBCATEGORIES: Array<{ key: AssistantSubcategory; label: string }> = [
  { key: 'text', label: 'Text' },
  { key: 'tool-use', label: 'Tool Use' },
  { key: 'thinking', label: 'Thinking' },
];

// Flat category type - categories without subcategories
type FlatCategory = 'system' | 'file-history-snapshot' | 'summary' | 'queue-operation';

// Flat categories (no subcategories)
interface FlatFilterItem {
  category: FlatCategory;
  label: string;
  color: SidebarItemColor;
}

const FLAT_CATEGORIES: FlatFilterItem[] = [
  { category: 'system', label: 'System', color: 'gray' },
  { category: 'file-history-snapshot', label: 'File Snapshot', color: 'cyan' },
  { category: 'summary', label: 'Summary', color: 'purple' },
  { category: 'queue-operation', label: 'Queue', color: 'amber' },
];

// Checkbox state types
type CheckboxState = 'checked' | 'unchecked' | 'indeterminate';

function FilterDropdown({ counts, filterState, onToggleCategory, onToggleUserSubcategory, onToggleAssistantSubcategory }: FilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Expand/collapse state for hierarchical categories
  const [expandedCategories, setExpandedCategories] = useState<Set<'user' | 'assistant'>>(new Set());

  const toggleExpand = (category: 'user' | 'assistant') => {
    setExpandedCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) {
        next.delete(category);
      } else {
        next.add(category);
      }
      return next;
    });
  };

  // Calculate checkbox state for hierarchical categories
  function getUserCheckboxState(): CheckboxState {
    const { prompt, 'tool-result': toolResult } = filterState.user;
    if (prompt && toolResult) return 'checked';
    if (!prompt && !toolResult) return 'unchecked';
    return 'indeterminate';
  }

  function getAssistantCheckboxState(): CheckboxState {
    const { text, 'tool-use': toolUse, thinking } = filterState.assistant;
    if (text && toolUse && thinking) return 'checked';
    if (!text && !toolUse && !thinking) return 'unchecked';
    return 'indeterminate';
  }

  // Check if filters are active (any category hidden that has messages)
  function hasActiveFilters(): boolean {
    // Check user subcategories
    if (counts.user.prompt > 0 && !filterState.user.prompt) return true;
    if (counts.user['tool-result'] > 0 && !filterState.user['tool-result']) return true;

    // Check assistant subcategories
    if (counts.assistant.text > 0 && !filterState.assistant.text) return true;
    if (counts.assistant['tool-use'] > 0 && !filterState.assistant['tool-use']) return true;
    if (counts.assistant.thinking > 0 && !filterState.assistant.thinking) return true;

    // Check flat categories
    if (counts.system > 0 && !filterState.system) return true;
    if (counts['file-history-snapshot'] > 0 && !filterState['file-history-snapshot']) return true;
    if (counts.summary > 0 && !filterState.summary) return true;
    if (counts['queue-operation'] > 0 && !filterState['queue-operation']) return true;

    return false;
  }

  const isUserExpanded = expandedCategories.has('user');
  const isAssistantExpanded = expandedCategories.has('assistant');
  const userCheckboxState = getUserCheckboxState();
  const assistantCheckboxState = getAssistantCheckboxState();

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        className={`${styles.filterBtn} ${hasActiveFilters() ? styles.active : ''}`}
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
            {/* User category with subcategories */}
            <div className={styles.categoryGroup}>
              <div className={`${styles.filterItem} ${styles.parentItem} ${counts.user.total === 0 ? styles.disabled : ''}`}>
                <button
                  className={styles.expandBtn}
                  onClick={() => toggleExpand('user')}
                  aria-label={isUserExpanded ? 'Collapse user subcategories' : 'Expand user subcategories'}
                >
                  <span className={`${styles.expandIcon} ${isUserExpanded ? styles.expanded : ''}`}>
                    <ChevronIcon />
                  </span>
                </button>
                <button
                  className={styles.checkboxBtn}
                  onClick={() => counts.user.total > 0 && onToggleCategory('user')}
                  disabled={counts.user.total === 0}
                  aria-label={`Toggle all user messages`}
                >
                  <span
                    className={`${styles.checkbox} ${styles[userCheckboxState]}`}
                    style={{ color: userCheckboxState !== 'unchecked' ? getColorValue('green') : undefined }}
                  >
                    {userCheckboxState === 'indeterminate' ? <MinusIcon /> : CheckIcon}
                  </span>
                  <span className={styles.filterLabel}>User</span>
                  <span className={styles.filterCount}>{counts.user.total}</span>
                </button>
              </div>

              {isUserExpanded && (
                <div className={styles.subcategories}>
                  {USER_SUBCATEGORIES.map((sub) => {
                    const count = counts.user[sub.key];
                    const isVisible = filterState.user[sub.key];
                    const isDisabled = count === 0;

                    return (
                      <button
                        key={sub.key}
                        className={`${styles.filterItem} ${styles.subcategoryItem} ${isDisabled ? styles.disabled : ''}`}
                        onClick={() => !isDisabled && onToggleUserSubcategory(sub.key)}
                        disabled={isDisabled}
                      >
                        <span
                          className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                          style={{ color: isVisible ? getColorValue('green') : undefined }}
                        >
                          {CheckIcon}
                        </span>
                        <span className={styles.filterLabel}>{sub.label}</span>
                        <span className={styles.filterCount}>{count}</span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Assistant category with subcategories */}
            <div className={styles.categoryGroup}>
              <div className={`${styles.filterItem} ${styles.parentItem} ${counts.assistant.total === 0 ? styles.disabled : ''}`}>
                <button
                  className={styles.expandBtn}
                  onClick={() => toggleExpand('assistant')}
                  aria-label={isAssistantExpanded ? 'Collapse assistant subcategories' : 'Expand assistant subcategories'}
                >
                  <span className={`${styles.expandIcon} ${isAssistantExpanded ? styles.expanded : ''}`}>
                    <ChevronIcon />
                  </span>
                </button>
                <button
                  className={styles.checkboxBtn}
                  onClick={() => counts.assistant.total > 0 && onToggleCategory('assistant')}
                  disabled={counts.assistant.total === 0}
                  aria-label={`Toggle all assistant messages`}
                >
                  <span
                    className={`${styles.checkbox} ${styles[assistantCheckboxState]}`}
                    style={{ color: assistantCheckboxState !== 'unchecked' ? getColorValue('blue') : undefined }}
                  >
                    {assistantCheckboxState === 'indeterminate' ? <MinusIcon /> : CheckIcon}
                  </span>
                  <span className={styles.filterLabel}>Assistant</span>
                  <span className={styles.filterCount}>{counts.assistant.total}</span>
                </button>
              </div>

              {isAssistantExpanded && (
                <div className={styles.subcategories}>
                  {ASSISTANT_SUBCATEGORIES.map((sub) => {
                    const count = counts.assistant[sub.key];
                    const isVisible = filterState.assistant[sub.key];
                    const isDisabled = count === 0;

                    return (
                      <button
                        key={sub.key}
                        className={`${styles.filterItem} ${styles.subcategoryItem} ${isDisabled ? styles.disabled : ''}`}
                        onClick={() => !isDisabled && onToggleAssistantSubcategory(sub.key)}
                        disabled={isDisabled}
                      >
                        <span
                          className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                          style={{ color: isVisible ? getColorValue('blue') : undefined }}
                        >
                          {CheckIcon}
                        </span>
                        <span className={styles.filterLabel}>{sub.label}</span>
                        <span className={styles.filterCount}>{count}</span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Flat categories */}
            {FLAT_CATEGORIES.map((item) => {
              const count = counts[item.category];
              const isVisible = filterState[item.category];
              const isDisabled = count === 0;

              return (
                <button
                  key={item.category}
                  className={`${styles.filterItem} ${styles.flatItem} ${isDisabled ? styles.disabled : ''}`}
                  onClick={() => !isDisabled && onToggleCategory(item.category)}
                  disabled={isDisabled}
                >
                  <span
                    className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                    style={{ color: isVisible ? getColorValue(item.color) : undefined }}
                  >
                    {CheckIcon}
                  </span>
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

// Chevron icon for expand/collapse
function ChevronIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

// Minus icon for indeterminate state
function MinusIcon() {
  return (
    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

export default FilterDropdown;
