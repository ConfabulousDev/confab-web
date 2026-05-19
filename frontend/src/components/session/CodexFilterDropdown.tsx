// CF-361: Codex transcript filter dropdown. Parallel to FilterDropdown.tsx
// but tuned to the Codex category model. Visually identical — both components
// import `FilterDropdownShared.module.css`.
//
// Hierarchy:
//   - assistant (commentary, final)
//   - tool_call (exec_command, apply_patch, web_search, generic)
// Flat:
//   - user, reasoning_hidden, compacted, turn_separator, unknown
//
// Zero-count chips render as disabled+greyed; parent rows show a tri-state
// checkbox (checked/unchecked/indeterminate).

import { useState } from 'react';
import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon } from '../icons';
import type {
  CodexCategory,
  CodexAssistantSubcategory,
  CodexToolCallSubcategory,
  CodexHierarchicalCounts,
  CodexFilterState,
} from './codexCategories';
import type { SidebarItemColor } from '../PageSidebar';
import styles from './FilterDropdownShared.module.css';

interface CodexFilterDropdownProps {
  counts: CodexHierarchicalCounts;
  filterState: CodexFilterState;
  onToggleCategory: (category: CodexCategory) => void;
  onToggleAssistantSubcategory: (sub: CodexAssistantSubcategory) => void;
  onToggleToolCallSubcategory: (sub: CodexToolCallSubcategory) => void;
}

const ASSISTANT_SUBCATEGORIES: Array<{ key: CodexAssistantSubcategory; label: string }> = [
  { key: 'commentary', label: 'Commentary' },
  { key: 'final', label: 'Final' },
];

const TOOL_CALL_SUBCATEGORIES: Array<{ key: CodexToolCallSubcategory; label: string }> = [
  { key: 'exec_command', label: 'Exec Command' },
  { key: 'apply_patch', label: 'Apply Patch' },
  { key: 'web_search', label: 'Web Search' },
  { key: 'generic', label: 'Other' },
];

type FlatCategory = Exclude<CodexCategory, 'assistant' | 'tool_call'>;

interface FlatFilterItem {
  category: FlatCategory;
  label: string;
  color: SidebarItemColor;
}

const FLAT_CATEGORIES: FlatFilterItem[] = [
  { category: 'user', label: 'User', color: 'green' },
  { category: 'reasoning_hidden', label: 'Reasoning Hidden', color: 'purple' },
  { category: 'compacted', label: 'Compacted', color: 'cyan' },
  { category: 'turn_separator', label: 'Turn Separator', color: 'gray' },
  // CF-368: aborted-turn divider — sits next to the regular turn separator
  // chip since both are turn-boundary markers; amber to mirror the
  // warning-coloured CSS on the divider itself.
  { category: 'turn_aborted', label: 'Turn Aborted', color: 'amber' },
  { category: 'unknown', label: 'Unknown', color: 'default' },
];

type CheckboxState = 'checked' | 'unchecked' | 'indeterminate';

function rollupCheckboxState(values: boolean[]): CheckboxState {
  if (values.every(Boolean)) return 'checked';
  if (values.every((v) => !v)) return 'unchecked';
  return 'indeterminate';
}

function CodexFilterDropdown({
  counts,
  filterState,
  onToggleCategory,
  onToggleAssistantSubcategory,
  onToggleToolCallSubcategory,
}: CodexFilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  type HierarchicalParent = 'assistant' | 'tool_call';
  const [expandedCategories, setExpandedCategories] = useState<Set<HierarchicalParent>>(new Set());

  const toggleExpand = (category: HierarchicalParent) => {
    setExpandedCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) next.delete(category);
      else next.add(category);
      return next;
    });
  };

  function hasActiveFilters(): boolean {
    const hiddenWithItems = (count: number, visible: boolean) => count > 0 && !visible;

    for (const sub of ASSISTANT_SUBCATEGORIES) {
      if (hiddenWithItems(counts.assistant[sub.key], filterState.assistant[sub.key])) return true;
    }
    for (const sub of TOOL_CALL_SUBCATEGORIES) {
      if (hiddenWithItems(counts.tool_call[sub.key], filterState.tool_call[sub.key])) return true;
    }
    for (const item of FLAT_CATEGORIES) {
      if (hiddenWithItems(counts[item.category], filterState[item.category])) return true;
    }
    return false;
  }

  const isAssistantExpanded = expandedCategories.has('assistant');
  const isToolCallExpanded = expandedCategories.has('tool_call');
  const assistantCheckboxState = rollupCheckboxState(
    ASSISTANT_SUBCATEGORIES.map((sub) => filterState.assistant[sub.key]),
  );
  const toolCallCheckboxState = rollupCheckboxState(
    TOOL_CALL_SUBCATEGORIES.map((sub) => filterState.tool_call[sub.key]),
  );

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
                  aria-label="Toggle all assistant messages"
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

            {/* Tool call category with subcategories */}
            <div className={styles.categoryGroup}>
              <div className={`${styles.filterItem} ${styles.parentItem} ${counts.tool_call.total === 0 ? styles.disabled : ''}`}>
                <button
                  className={styles.expandBtn}
                  onClick={() => toggleExpand('tool_call')}
                  aria-label={isToolCallExpanded ? 'Collapse tool call subcategories' : 'Expand tool call subcategories'}
                >
                  <span className={`${styles.expandIcon} ${isToolCallExpanded ? styles.expanded : ''}`}>
                    <ChevronIcon />
                  </span>
                </button>
                <button
                  className={styles.checkboxBtn}
                  onClick={() => counts.tool_call.total > 0 && onToggleCategory('tool_call')}
                  disabled={counts.tool_call.total === 0}
                  aria-label="Toggle all tool calls"
                >
                  <span
                    className={`${styles.checkbox} ${styles[toolCallCheckboxState]}`}
                    style={{ color: toolCallCheckboxState !== 'unchecked' ? getColorValue('amber') : undefined }}
                  >
                    {toolCallCheckboxState === 'indeterminate' ? <MinusIcon /> : CheckIcon}
                  </span>
                  <span className={styles.filterLabel}>Tool Call</span>
                  <span className={styles.filterCount}>{counts.tool_call.total}</span>
                </button>
              </div>

              {isToolCallExpanded && (
                <div className={styles.subcategories}>
                  {TOOL_CALL_SUBCATEGORIES.map((sub) => {
                    const count = counts.tool_call[sub.key];
                    const isVisible = filterState.tool_call[sub.key];
                    const isDisabled = count === 0;

                    return (
                      <button
                        key={sub.key}
                        className={`${styles.filterItem} ${styles.subcategoryItem} ${isDisabled ? styles.disabled : ''}`}
                        onClick={() => !isDisabled && onToggleToolCallSubcategory(sub.key)}
                        disabled={isDisabled}
                      >
                        <span
                          className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                          style={{ color: isVisible ? getColorValue('amber') : undefined }}
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

function ChevronIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

function MinusIcon() {
  return (
    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

export default CodexFilterDropdown;
