// Cursor transcript filter dropdown (MVP, 18n2). A thin wrapper that assembles
// flat chips (no subcategories) for the shared ProviderFilterDropdown (ew9f),
// mirroring OpenCodeFilterDropdown.
//
// Zero-count chips render disabled+greyed; the toggle hides/shows that
// category's rows in the transcript pane.

import type {
  CursorCategory,
  CursorFilterState,
  CursorHierarchicalCounts,
} from './cursorCategories';
import ProviderFilterDropdown from './ProviderFilterDropdown';
import type { FilterChip } from './filterChips';

interface CursorFilterDropdownProps {
  counts: CursorHierarchicalCounts;
  filterState: CursorFilterState;
  onToggleCategory: (category: CursorCategory) => void;
}

const CATEGORIES: Array<{ category: CursorCategory; label: string; color: string }> = [
  { category: 'user', label: 'User', color: '#16a34a' },
  { category: 'assistant', label: 'Assistant', color: '#2563eb' },
  { category: 'tool', label: 'Tool Call', color: '#d97706' },
];

export default function CursorFilterDropdown({
  counts,
  filterState,
  onToggleCategory,
}: CursorFilterDropdownProps) {
  const flatItems: FilterChip[] = CATEGORIES.map(({ category, label, color }) => ({
    key: category,
    label,
    count: counts[category],
    visible: filterState[category],
    color,
    onToggle: () => onToggleCategory(category),
  }));

  return <ProviderFilterDropdown groups={[]} flatItems={flatItems} />;
}
