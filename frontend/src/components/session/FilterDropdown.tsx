import { useState } from 'react';
import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon } from '../icons';
import type {
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  AttachmentSubcategory,
  HierarchicalCounts,
  FilterState,
} from './messageCategories';
import type { SidebarItemColor } from '../PageSidebar';
import styles from './FilterDropdownShared.module.css';

interface FilterDropdownProps {
  counts: HierarchicalCounts;
  filterState: FilterState;
  onToggleCategory: (category: MessageCategory) => void;
  onToggleUserSubcategory: (subcategory: UserSubcategory) => void;
  onToggleAssistantSubcategory: (subcategory: AssistantSubcategory) => void;
  onToggleAttachmentSubcategory: (subcategory: AttachmentSubcategory) => void;
}

// Subcategory configurations
const USER_SUBCATEGORIES: Array<{ key: UserSubcategory; label: string }> = [
  { key: 'prompt', label: 'Prompts' },
  { key: 'tool-result', label: 'Tool Results' },
  { key: 'skill', label: 'Skills' },
];

const ASSISTANT_SUBCATEGORIES: Array<{ key: AssistantSubcategory; label: string }> = [
  { key: 'text', label: 'Text' },
  { key: 'tool-use', label: 'Tool Use' },
  { key: 'thinking', label: 'Thinking' },
];

const ATTACHMENT_SUBCATEGORIES: Array<{ key: AttachmentSubcategory; label: string }> = [
  { key: 'hook', label: 'Hook' },
  { key: 'file-edit', label: 'File Edit' },
  { key: 'queued-command', label: 'Queued Command' },
  { key: 'deferred-tools', label: 'Deferred Tools' },
  { key: 'mcp-instructions', label: 'MCP Instructions' },
];

// Flat category type - categories without subcategories
type FlatCategory = 'system' | 'file-history-snapshot' | 'summary' | 'queue-operation' | 'pr-link' | 'away-summary';

// Flat categories (no subcategories)
interface FlatFilterItem {
  category: FlatCategory;
  label: string;
  color: SidebarItemColor;
}

const FLAT_CATEGORIES: FlatFilterItem[] = [
  { category: 'system', label: 'System', color: 'gray' },
  { category: 'away-summary', label: 'Resume Summary', color: 'purple' },
  { category: 'file-history-snapshot', label: 'File Snapshot', color: 'cyan' },
  { category: 'summary', label: 'Summary', color: 'purple' },
  { category: 'queue-operation', label: 'Queue', color: 'amber' },
  { category: 'pr-link', label: 'PR Link', color: 'green' },
];

// Checkbox state types
type CheckboxState = 'checked' | 'unchecked' | 'indeterminate';

function FilterDropdown({ counts, filterState, onToggleCategory, onToggleUserSubcategory, onToggleAssistantSubcategory, onToggleAttachmentSubcategory }: FilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Expand/collapse state for hierarchical categories
  type HierarchicalParent = 'user' | 'assistant' | 'attachment';
  const [expandedCategories, setExpandedCategories] = useState<Set<HierarchicalParent>>(new Set());

  const toggleExpand = (category: HierarchicalParent) => {
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

  // Roll up a set of booleans into a parent checkbox tri-state.
  function rollupCheckboxState(values: boolean[]): CheckboxState {
    if (values.every(Boolean)) return 'checked';
    if (values.every((v) => !v)) return 'unchecked';
    return 'indeterminate';
  }

  // Check if filters are active (any category hidden that has messages)
  function hasActiveFilters(): boolean {
    const hiddenWithMessages = (count: number, visible: boolean) => count > 0 && !visible;

    for (const sub of USER_SUBCATEGORIES) {
      if (hiddenWithMessages(counts.user[sub.key], filterState.user[sub.key])) return true;
    }
    for (const sub of ASSISTANT_SUBCATEGORIES) {
      if (hiddenWithMessages(counts.assistant[sub.key], filterState.assistant[sub.key])) return true;
    }
    for (const sub of ATTACHMENT_SUBCATEGORIES) {
      if (hiddenWithMessages(counts.attachment[sub.key], filterState.attachment[sub.key])) return true;
    }
    for (const item of FLAT_CATEGORIES) {
      if (hiddenWithMessages(counts[item.category], filterState[item.category])) return true;
    }
    return false;
  }

  const isUserExpanded = expandedCategories.has('user');
  const isAssistantExpanded = expandedCategories.has('assistant');
  const isAttachmentExpanded = expandedCategories.has('attachment');
  const userCheckboxState = rollupCheckboxState(
    USER_SUBCATEGORIES.map((sub) => filterState.user[sub.key]),
  );
  const assistantCheckboxState = rollupCheckboxState(
    ASSISTANT_SUBCATEGORIES.map((sub) => filterState.assistant[sub.key]),
  );
  const attachmentCheckboxState = rollupCheckboxState(
    ATTACHMENT_SUBCATEGORIES.map((sub) => filterState.attachment[sub.key]),
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

            {/* Attachment category with subcategories (CF-346) */}
            <div className={styles.categoryGroup}>
              <div className={`${styles.filterItem} ${styles.parentItem} ${counts.attachment.total === 0 ? styles.disabled : ''}`}>
                <button
                  className={styles.expandBtn}
                  onClick={() => toggleExpand('attachment')}
                  aria-label={isAttachmentExpanded ? 'Collapse attachment subcategories' : 'Expand attachment subcategories'}
                >
                  <span className={`${styles.expandIcon} ${isAttachmentExpanded ? styles.expanded : ''}`}>
                    <ChevronIcon />
                  </span>
                </button>
                <button
                  className={styles.checkboxBtn}
                  onClick={() => counts.attachment.total > 0 && onToggleCategory('attachment')}
                  disabled={counts.attachment.total === 0}
                  aria-label="Toggle all attachment messages"
                >
                  <span
                    className={`${styles.checkbox} ${styles[attachmentCheckboxState]}`}
                    style={{ color: attachmentCheckboxState !== 'unchecked' ? getColorValue('gray') : undefined }}
                  >
                    {attachmentCheckboxState === 'indeterminate' ? <MinusIcon /> : CheckIcon}
                  </span>
                  <span className={styles.filterLabel}>Attachment</span>
                  <span className={styles.filterCount}>{counts.attachment.total}</span>
                </button>
              </div>

              {isAttachmentExpanded && (
                <div className={styles.subcategories}>
                  {ATTACHMENT_SUBCATEGORIES.map((sub) => {
                    const count = counts.attachment[sub.key];
                    const isVisible = filterState.attachment[sub.key];
                    const isDisabled = count === 0;

                    return (
                      <button
                        key={sub.key}
                        className={`${styles.filterItem} ${styles.subcategoryItem} ${isDisabled ? styles.disabled : ''}`}
                        onClick={() => !isDisabled && onToggleAttachmentSubcategory(sub.key)}
                        disabled={isDisabled}
                      >
                        <span
                          className={`${styles.checkbox} ${isVisible ? styles.checked : ''}`}
                          style={{ color: isVisible ? getColorValue('gray') : undefined }}
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
