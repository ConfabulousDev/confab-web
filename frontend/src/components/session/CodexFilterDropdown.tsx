// CF-361: Codex transcript filter dropdown. A thin wrapper that assembles the
// Codex category model (two hierarchical groups + flat chips) for the shared
// ProviderFilterDropdown (ew9f).
//
// Hierarchy:
//   - assistant (commentary, final)
//   - tool_call (exec_command, apply_patch, web_search, generic)
// Flat:
//   - user, reasoning_hidden, compacted, turn_separator, turn_aborted, unknown

import type {
  CodexCategory,
  CodexAssistantSubcategory,
  CodexToolCallSubcategory,
  CodexHierarchicalCounts,
  CodexFilterState,
} from './codexCategories';
import type { SidebarItemColor } from '../PageSidebar';
import ProviderFilterDropdown from './ProviderFilterDropdown';
import { getColorValue, type FilterChip, type FilterChipGroup } from './filterChips';

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

const FLAT_CATEGORIES: Array<{ category: FlatCategory; label: string; color: SidebarItemColor }> = [
  { category: 'user', label: 'User', color: 'green' },
  { category: 'reasoning_hidden', label: 'Reasoning Hidden', color: 'purple' },
  { category: 'compacted', label: 'Compacted', color: 'cyan' },
  { category: 'turn_separator', label: 'Turn Separator', color: 'gray' },
  // CF-368: aborted-turn divider — amber to mirror the warning-coloured divider.
  { category: 'turn_aborted', label: 'Turn Aborted', color: 'amber' },
  { category: 'unknown', label: 'Unknown', color: 'default' },
];

export default function CodexFilterDropdown({
  counts,
  filterState,
  onToggleCategory,
  onToggleAssistantSubcategory,
  onToggleToolCallSubcategory,
}: CodexFilterDropdownProps) {
  const groups: FilterChipGroup[] = [
    {
      key: 'assistant',
      label: 'Assistant',
      total: counts.assistant.total,
      color: getColorValue('blue'),
      expandNoun: 'assistant subcategories',
      toggleAllLabel: 'Toggle all assistant messages',
      onToggleParent: () => onToggleCategory('assistant'),
      subItems: ASSISTANT_SUBCATEGORIES.map((sub) => ({
        key: sub.key,
        label: sub.label,
        count: counts.assistant[sub.key],
        visible: filterState.assistant[sub.key],
        color: getColorValue('blue'),
        onToggle: () => onToggleAssistantSubcategory(sub.key),
      })),
    },
    {
      key: 'tool_call',
      label: 'Tool Call',
      total: counts.tool_call.total,
      color: getColorValue('amber'),
      expandNoun: 'tool call subcategories',
      toggleAllLabel: 'Toggle all tool calls',
      onToggleParent: () => onToggleCategory('tool_call'),
      subItems: TOOL_CALL_SUBCATEGORIES.map((sub) => ({
        key: sub.key,
        label: sub.label,
        count: counts.tool_call[sub.key],
        visible: filterState.tool_call[sub.key],
        color: getColorValue('amber'),
        onToggle: () => onToggleToolCallSubcategory(sub.key),
      })),
    },
  ];

  const flatItems: FilterChip[] = FLAT_CATEGORIES.map((item) => ({
    key: item.category,
    label: item.label,
    count: counts[item.category],
    visible: filterState[item.category],
    color: getColorValue(item.color),
    onToggle: () => onToggleCategory(item.category),
  }));

  return <ProviderFilterDropdown groups={groups} flatItems={flatItems} />;
}
