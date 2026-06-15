// Claude transcript filter dropdown. A thin wrapper that assembles the Claude
// category model (three hierarchical groups + flat chips) for the shared
// ProviderFilterDropdown (ew9f).
//
// Hierarchy: user / assistant / attachment (each with subcategories).
// Flat: system, away-summary, file-history-snapshot, summary, queue-operation, pr-link.

import type {
  ClaudeCategory,
  ClaudeUserSubcategory,
  ClaudeAssistantSubcategory,
  ClaudeAttachmentSubcategory,
  ClaudeHierarchicalCounts,
  ClaudeFilterState,
} from './claudeCategories';
import type { SidebarItemColor } from '../PageSidebar';
import ProviderFilterDropdown from './ProviderFilterDropdown';
import { getColorValue, type FilterChip, type FilterChipGroup } from './filterChips';

interface ClaudeFilterDropdownProps {
  counts: ClaudeHierarchicalCounts;
  filterState: ClaudeFilterState;
  onToggleCategory: (category: ClaudeCategory) => void;
  onToggleUserSubcategory: (subcategory: ClaudeUserSubcategory) => void;
  onToggleAssistantSubcategory: (subcategory: ClaudeAssistantSubcategory) => void;
  onToggleAttachmentSubcategory: (subcategory: ClaudeAttachmentSubcategory) => void;
}

const USER_SUBCATEGORIES: Array<{ key: ClaudeUserSubcategory; label: string }> = [
  { key: 'prompt', label: 'Prompts' },
  { key: 'tool-result', label: 'Tool Results' },
  { key: 'skill', label: 'Skills' },
];

const ASSISTANT_SUBCATEGORIES: Array<{ key: ClaudeAssistantSubcategory; label: string }> = [
  { key: 'text', label: 'Text' },
  { key: 'tool-use', label: 'Tool Use' },
  { key: 'thinking', label: 'Thinking' },
];

const ATTACHMENT_SUBCATEGORIES: Array<{ key: ClaudeAttachmentSubcategory; label: string }> = [
  { key: 'hook', label: 'Hook' },
  { key: 'file-edit', label: 'File Edit' },
  { key: 'queued-command', label: 'Queued Command' },
  { key: 'deferred-tools', label: 'Deferred Tools' },
  { key: 'mcp-instructions', label: 'MCP Instructions' },
];

type FlatCategory = 'system' | 'file-history-snapshot' | 'summary' | 'queue-operation' | 'pr-link' | 'away-summary';

const FLAT_CATEGORIES: Array<{ category: FlatCategory; label: string; color: SidebarItemColor }> = [
  { category: 'system', label: 'System', color: 'gray' },
  { category: 'away-summary', label: 'Resume Summary', color: 'purple' },
  { category: 'file-history-snapshot', label: 'File Snapshot', color: 'cyan' },
  { category: 'summary', label: 'Summary', color: 'purple' },
  { category: 'queue-operation', label: 'Queue', color: 'amber' },
  { category: 'pr-link', label: 'PR Link', color: 'green' },
];

export default function ClaudeFilterDropdown({
  counts,
  filterState,
  onToggleCategory,
  onToggleUserSubcategory,
  onToggleAssistantSubcategory,
  onToggleAttachmentSubcategory,
}: ClaudeFilterDropdownProps) {
  const groups: FilterChipGroup[] = [
    {
      key: 'user',
      label: 'User',
      total: counts.user.total,
      color: getColorValue('green'),
      expandNoun: 'user subcategories',
      toggleAllLabel: 'Toggle all user messages',
      onToggleParent: () => onToggleCategory('user'),
      subItems: USER_SUBCATEGORIES.map((sub) => ({
        key: sub.key,
        label: sub.label,
        count: counts.user[sub.key],
        visible: filterState.user[sub.key],
        color: getColorValue('green'),
        onToggle: () => onToggleUserSubcategory(sub.key),
      })),
    },
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
      key: 'attachment',
      label: 'Attachment',
      total: counts.attachment.total,
      color: getColorValue('gray'),
      expandNoun: 'attachment subcategories',
      toggleAllLabel: 'Toggle all attachment messages',
      onToggleParent: () => onToggleCategory('attachment'),
      subItems: ATTACHMENT_SUBCATEGORIES.map((sub) => ({
        key: sub.key,
        label: sub.label,
        count: counts.attachment[sub.key],
        visible: filterState.attachment[sub.key],
        color: getColorValue('gray'),
        onToggle: () => onToggleAttachmentSubcategory(sub.key),
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
