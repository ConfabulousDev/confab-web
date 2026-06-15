// OpenCode transcript filter dropdown (MVP). A thin wrapper that assembles flat
// chips (no subcategories) for the shared ProviderFilterDropdown (ew9f).
//
// Zero-count chips render disabled+greyed; the toggle hides/shows that
// category's rows in the transcript pane.

import type {
  OpenCodeCategory,
  OpenCodeFilterState,
  OpenCodeHierarchicalCounts,
} from './opencodeCategories';
import ProviderFilterDropdown from './ProviderFilterDropdown';
import type { FilterChip } from './filterChips';

interface OpenCodeFilterDropdownProps {
  counts: OpenCodeHierarchicalCounts;
  filterState: OpenCodeFilterState;
  onToggleCategory: (category: OpenCodeCategory) => void;
}

const CATEGORIES: Array<{ category: OpenCodeCategory; label: string; color: string }> = [
  { category: 'user', label: 'User', color: '#16a34a' },
  { category: 'assistant', label: 'Assistant', color: '#2563eb' },
  { category: 'tool', label: 'Tool Call', color: '#d97706' },
  { category: 'unknown', label: 'Unknown', color: '#dc2626' },
];

export default function OpenCodeFilterDropdown({
  counts,
  filterState,
  onToggleCategory,
}: OpenCodeFilterDropdownProps) {
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
