// Shared chip types + color helper for the generic ProviderFilterDropdown (ew9f).
// Kept separate from the component file so it can export non-component values
// without tripping react-refresh's only-export-components rule.

import type { SidebarItemColor } from '../PageSidebar';

export type CheckboxState = 'checked' | 'unchecked' | 'indeterminate';

/** A single leaf chip — a flat category or a subcategory. `color` is a hex string. */
export interface FilterChip {
  key: string;
  label: string;
  count: number;
  visible: boolean;
  color: string;
  onToggle: () => void;
}

/** A hierarchical category: a parent row (tri-state, expand/collapse) over subcategory chips. */
export interface FilterChipGroup {
  key: string;
  label: string;
  total: number;
  /** Hex color for the parent checkbox when not fully unchecked. */
  color: string;
  /** Noun for the expand/collapse aria-label, e.g. `"assistant subcategories"`. */
  expandNoun: string;
  /** Full aria-label for the parent toggle, e.g. `"Toggle all assistant messages"`. */
  toggleAllLabel: string;
  onToggleParent: () => void;
  subItems: FilterChip[];
}

const SIDEBAR_COLOR_HEX: Record<SidebarItemColor, string> = {
  default: '#2563eb',
  green: '#16a34a',
  blue: '#2563eb',
  gray: '#6b7280',
  cyan: '#0284c7',
  purple: '#7c3aed',
  amber: '#d97706',
};

/** Resolve a SidebarItemColor token to its hex value (shared by Claude/Codex assemblers). */
export function getColorValue(color: SidebarItemColor): string {
  return SIDEBAR_COLOR_HEX[color];
}
