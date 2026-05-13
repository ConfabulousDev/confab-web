import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import FilterDropdown from './FilterDropdown';
import type {
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  AttachmentSubcategory,
  HierarchicalCounts,
  FilterState,
} from './messageCategories';
import { DEFAULT_FILTER_STATE } from './messageCategories';

// Sample hierarchical counts for stories
const sampleCounts: HierarchicalCounts = {
  user: { total: 194, prompt: 40, 'tool-result': 152, skill: 2 },
  assistant: { total: 271, text: 50, 'tool-use': 180, thinking: 41 },
  attachment: { total: 0, hook: 0, 'file-edit': 0, 'queued-command': 0, 'deferred-tools': 0, 'mcp-instructions': 0 },
  system: 0,
  'file-history-snapshot': 39,
  summary: 0,
  'queue-operation': 6,
  'pr-link': 0,
  'away-summary': 0,
  unknown: 0,
};

// Interactive wrapper component
function FilterDropdownInteractive({
  initialFilterState = DEFAULT_FILTER_STATE,
  counts = sampleCounts,
}: {
  initialFilterState?: FilterState;
  counts?: HierarchicalCounts;
}) {
  const [filterState, setFilterState] = useState(initialFilterState);

  const handleToggleCategory = (category: MessageCategory) => {
    setFilterState((prev) => {
      const next = { ...prev };
      if (category === 'user') {
        const allVisible = prev.user.prompt && prev.user['tool-result'] && prev.user.skill;
        next.user = { prompt: !allVisible, 'tool-result': !allVisible, skill: !allVisible };
      } else if (category === 'assistant') {
        const allVisible = prev.assistant.text && prev.assistant['tool-use'] && prev.assistant.thinking;
        next.assistant = { text: !allVisible, 'tool-use': !allVisible, thinking: !allVisible };
      } else if (category === 'attachment') {
        const a = prev.attachment;
        const allVisible = a.hook && a['file-edit'] && a['queued-command'] && a['deferred-tools'] && a['mcp-instructions'];
        next.attachment = {
          hook: !allVisible,
          'file-edit': !allVisible,
          'queued-command': !allVisible,
          'deferred-tools': !allVisible,
          'mcp-instructions': !allVisible,
        };
      } else {
        next[category] = !prev[category];
      }
      return next;
    });
  };

  const handleToggleUserSubcategory = (subcategory: UserSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      user: { ...prev.user, [subcategory]: !prev.user[subcategory] },
    }));
  };

  const handleToggleAssistantSubcategory = (subcategory: AssistantSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      assistant: { ...prev.assistant, [subcategory]: !prev.assistant[subcategory] },
    }));
  };

  const handleToggleAttachmentSubcategory = (subcategory: AttachmentSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      attachment: { ...prev.attachment, [subcategory]: !prev.attachment[subcategory] },
    }));
  };

  return (
    <FilterDropdown
      counts={counts}
      filterState={filterState}
      onToggleCategory={handleToggleCategory}
      onToggleUserSubcategory={handleToggleUserSubcategory}
      onToggleAssistantSubcategory={handleToggleAssistantSubcategory}
      onToggleAttachmentSubcategory={handleToggleAttachmentSubcategory}
    />
  );
}

const meta: Meta<typeof FilterDropdownInteractive> = {
  title: 'Session/FilterDropdown',
  component: FilterDropdownInteractive,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ padding: '100px', background: 'var(--color-bg)' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof FilterDropdownInteractive>;

export const Default: Story = {
  args: {},
};

export const AllFiltersActive: Story = {
  args: {
    initialFilterState: {
      user: { prompt: true, 'tool-result': true, skill: true },
      assistant: { text: true, 'tool-use': true, thinking: true },
      attachment: { hook: true, 'file-edit': true, 'queued-command': true, 'deferred-tools': true, 'mcp-instructions': true },
      system: true,
      'file-history-snapshot': true,
      summary: true,
      'queue-operation': true,
      'pr-link': true,
      'away-summary': true,
      unknown: true,
    },
  },
};

export const SomeFiltersHidden: Story = {
  args: {
    initialFilterState: {
      user: { prompt: true, 'tool-result': false, skill: true },
      assistant: { text: true, 'tool-use': true, thinking: false },
      attachment: { hook: false, 'file-edit': false, 'queued-command': false, 'deferred-tools': false, 'mcp-instructions': false },
      system: false,
      'file-history-snapshot': false,
      summary: false,
      'queue-operation': false,
      'pr-link': false,
      'away-summary': false,
      unknown: true,
    },
  },
};

export const IndeterminateState: Story = {
  args: {
    initialFilterState: {
      user: { prompt: true, 'tool-result': false, skill: true }, // indeterminate
      assistant: { text: false, 'tool-use': true, thinking: false }, // indeterminate
      attachment: { hook: true, 'file-edit': false, 'queued-command': false, 'deferred-tools': true, 'mcp-instructions': false }, // indeterminate
      system: true,
      'file-history-snapshot': false,
      summary: false,
      'queue-operation': false,
      'pr-link': false,
      'away-summary': false,
      unknown: true,
    },
  },
};

export const AllCategoriesHaveMessages: Story = {
  args: {
    counts: {
      user: { total: 195, prompt: 40, 'tool-result': 152, skill: 3 },
      assistant: { total: 200, text: 80, 'tool-use': 100, thinking: 20 },
      attachment: { total: 18, hook: 8, 'file-edit': 3, 'queued-command': 2, 'deferred-tools': 3, 'mcp-instructions': 2 },
      system: 25,
      'file-history-snapshot': 40,
      summary: 5,
      'queue-operation': 10,
      'pr-link': 2,
      'away-summary': 1,
      unknown: 0,
    },
  },
};
