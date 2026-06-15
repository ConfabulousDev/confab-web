// ew9f: generic transcript filter dropdown shared by all providers.
//
// Claude/Codex/OpenCode previously each hand-rolled the same dropdown shell
// (filter button, expand/collapse hierarchical groups with tri-state parent
// checkboxes, flat chips). This component owns all of that rendering; each
// provider's thin wrapper just assembles `groups` + `flatItems` (resolving
// counts/visibility/colors/toggles) and renders this once.

import { useState } from 'react';
import { useDropdown } from '@/hooks';
import { FilterIcon, CheckIcon } from '../icons';
import styles from './FilterDropdownShared.module.css';
import type { CheckboxState, FilterChip, FilterChipGroup } from './filterChips';

interface ProviderFilterDropdownProps {
  groups: FilterChipGroup[];
  flatItems: FilterChip[];
}

function rollupCheckboxState(values: boolean[]): CheckboxState {
  if (values.every(Boolean)) return 'checked';
  if (values.every((v) => !v)) return 'unchecked';
  return 'indeterminate';
}

function Chip({ chip, variant }: { chip: FilterChip; variant: 'flat' | 'subcategory' }) {
  const isDisabled = chip.count === 0;
  const variantClass = variant === 'flat' ? styles.flatItem : styles.subcategoryItem;
  return (
    <button
      className={`${styles.filterItem} ${variantClass} ${isDisabled ? styles.disabled : ''}`}
      onClick={() => !isDisabled && chip.onToggle()}
      disabled={isDisabled}
    >
      <span
        className={`${styles.checkbox} ${chip.visible ? styles.checked : ''}`}
        style={{ color: chip.visible ? chip.color : undefined }}
      >
        {CheckIcon}
      </span>
      <span className={styles.filterLabel}>{chip.label}</span>
      <span className={styles.filterCount}>{chip.count}</span>
    </button>
  );
}

function CategoryGroup({
  group,
  expanded,
  onToggleExpand,
}: {
  group: FilterChipGroup;
  expanded: boolean;
  onToggleExpand: () => void;
}) {
  const parentState = rollupCheckboxState(group.subItems.map((s) => s.visible));
  const isDisabled = group.total === 0;
  return (
    <div className={styles.categoryGroup}>
      <div className={`${styles.filterItem} ${styles.parentItem} ${isDisabled ? styles.disabled : ''}`}>
        <button
          className={styles.expandBtn}
          onClick={onToggleExpand}
          aria-label={expanded ? `Collapse ${group.expandNoun}` : `Expand ${group.expandNoun}`}
        >
          <span className={`${styles.expandIcon} ${expanded ? styles.expanded : ''}`}>
            <ChevronIcon />
          </span>
        </button>
        <button
          className={styles.checkboxBtn}
          onClick={() => group.total > 0 && group.onToggleParent()}
          disabled={isDisabled}
          aria-label={group.toggleAllLabel}
        >
          <span
            className={`${styles.checkbox} ${styles[parentState]}`}
            style={{ color: parentState !== 'unchecked' ? group.color : undefined }}
          >
            {parentState === 'indeterminate' ? <MinusIcon /> : CheckIcon}
          </span>
          <span className={styles.filterLabel}>{group.label}</span>
          <span className={styles.filterCount}>{group.total}</span>
        </button>
      </div>

      {expanded && (
        <div className={styles.subcategories}>
          {group.subItems.map((chip) => (
            <Chip key={chip.key} chip={chip} variant="subcategory" />
          ))}
        </div>
      )}
    </div>
  );
}

export default function ProviderFilterDropdown({ groups, flatItems }: ProviderFilterDropdownProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());

  const toggleExpand = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  // Active = any leaf chip with rows but hidden (parents are derived, not counted).
  const leafChips = [...groups.flatMap((g) => g.subItems), ...flatItems];
  const hasActiveFilters = leafChips.some((c) => c.count > 0 && !c.visible);

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
            {groups.map((group) => (
              <CategoryGroup
                key={group.key}
                group={group}
                expanded={expandedGroups.has(group.key)}
                onToggleExpand={() => toggleExpand(group.key)}
              />
            ))}
            {flatItems.map((chip) => (
              <Chip key={chip.key} chip={chip} variant="flat" />
            ))}
          </div>
        </div>
      )}
    </div>
  );
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
