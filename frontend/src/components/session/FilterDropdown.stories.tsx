import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import FilterDropdown from './FilterDropdown';
import type { MessageCategory, MessageCategoryCounts } from './messageCategories';

// Sample counts for stories
const sampleCounts: MessageCategoryCounts = {
  user: 194,
  assistant: 271,
  system: 0,
  'file-history-snapshot': 39,
  summary: 0,
  'queue-operation': 6,
};

const defaultVisibleCategories = new Set<MessageCategory>([
  'user',
  'assistant',
  'system',
]);

// Interactive wrapper component
function FilterDropdownInteractive({
  initialVisible = defaultVisibleCategories,
  counts = sampleCounts,
}: {
  initialVisible?: Set<MessageCategory>;
  counts?: MessageCategoryCounts;
}) {
  const [visibleCategories, setVisibleCategories] = useState(initialVisible);

  const handleToggle = (category: MessageCategory) => {
    setVisibleCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) {
        next.delete(category);
      } else {
        next.add(category);
      }
      return next;
    });
  };

  return (
    <FilterDropdown
      counts={counts}
      visibleCategories={visibleCategories}
      onToggleCategory={handleToggle}
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
      <div style={{ padding: '100px', background: '#fafafa' }}>
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
    initialVisible: new Set([
      'user',
      'assistant',
      'system',
      'file-history-snapshot',
      'summary',
      'queue-operation',
    ]),
  },
};

export const SomeFiltersHidden: Story = {
  args: {
    initialVisible: new Set(['user', 'assistant']),
  },
};

export const AllCategoriesHaveMessages: Story = {
  args: {
    counts: {
      user: 150,
      assistant: 200,
      system: 25,
      'file-history-snapshot': 40,
      summary: 5,
      'queue-operation': 10,
    },
  },
};
