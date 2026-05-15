import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import CodexFilterDropdown from './CodexFilterDropdown';
import {
  DEFAULT_CODEX_FILTER_STATE,
  type CodexCategory,
  type CodexAssistantSubcategory,
  type CodexToolCallSubcategory,
  type CodexFilterState,
  type CodexHierarchicalCounts,
} from './codexCategories';

const sampleCounts: CodexHierarchicalCounts = {
  user: 12,
  assistant: { total: 21, commentary: 9, final: 12 },
  tool_call: { total: 17, exec_command: 11, apply_patch: 3, web_search: 1, generic: 2 },
  reasoning_hidden: 7,
  compacted: 1,
  turn_separator: 12,
  unknown: 0,
};

const zeroCounts: CodexHierarchicalCounts = {
  user: 0,
  assistant: { total: 0, commentary: 0, final: 0 },
  tool_call: { total: 0, exec_command: 0, apply_patch: 0, web_search: 0, generic: 0 },
  reasoning_hidden: 0,
  compacted: 0,
  turn_separator: 0,
  unknown: 0,
};

function Interactive({
  initialState = DEFAULT_CODEX_FILTER_STATE,
  counts = sampleCounts,
}: {
  initialState?: CodexFilterState;
  counts?: CodexHierarchicalCounts;
}) {
  const [filterState, setFilterState] = useState<CodexFilterState>(initialState);

  const onToggleCategory = (c: CodexCategory) => {
    setFilterState((prev) => {
      const next = { ...prev };
      if (c === 'assistant') {
        const all = prev.assistant.commentary && prev.assistant.final;
        next.assistant = { commentary: !all, final: !all };
      } else if (c === 'tool_call') {
        const tc = prev.tool_call;
        const all = tc.exec_command && tc.apply_patch && tc.web_search && tc.generic;
        next.tool_call = { exec_command: !all, apply_patch: !all, web_search: !all, generic: !all };
      } else {
        next[c] = !prev[c];
      }
      return next;
    });
  };

  const onToggleAssistantSubcategory = (sub: CodexAssistantSubcategory) =>
    setFilterState((prev) => ({
      ...prev,
      assistant: { ...prev.assistant, [sub]: !prev.assistant[sub] },
    }));

  const onToggleToolCallSubcategory = (sub: CodexToolCallSubcategory) =>
    setFilterState((prev) => ({
      ...prev,
      tool_call: { ...prev.tool_call, [sub]: !prev.tool_call[sub] },
    }));

  return (
    <div style={{ padding: 20, display: 'flex', justifyContent: 'flex-end' }}>
      <CodexFilterDropdown
        counts={counts}
        filterState={filterState}
        onToggleCategory={onToggleCategory}
        onToggleAssistantSubcategory={onToggleAssistantSubcategory}
        onToggleToolCallSubcategory={onToggleToolCallSubcategory}
      />
    </div>
  );
}

const meta: Meta<typeof Interactive> = {
  title: 'Session/CodexFilterDropdown',
  component: Interactive,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <div style={{ background: 'var(--color-bg)', minHeight: '600px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof Interactive>;

/** Default state — only `reasoning_hidden` hidden. */
export const Default: Story = {
  args: { initialState: DEFAULT_CODEX_FILTER_STATE, counts: sampleCounts },
};

/** All categories hidden — every parent shows the indeterminate-then-unchecked
 * roll-up; the chip button is visibly active. */
export const AllHidden: Story = {
  args: {
    counts: sampleCounts,
    initialState: {
      user: false,
      assistant: { commentary: false, final: false },
      tool_call: { exec_command: false, apply_patch: false, web_search: false, generic: false },
      reasoning_hidden: false,
      compacted: false,
      turn_separator: false,
      unknown: false,
    },
  },
};

/** Zero counts everywhere — every chip rendered disabled+greyed. */
export const ZeroCounts: Story = {
  args: {
    counts: zeroCounts,
    initialState: DEFAULT_CODEX_FILTER_STATE,
  },
};

/** Mixed partial state — assistant.commentary hidden while assistant.final is on;
 * tool_call.exec_command hidden while the rest are on. Drives the
 * indeterminate parent checkbox visual. */
export const PartialHidden: Story = {
  args: {
    counts: sampleCounts,
    initialState: {
      user: true,
      assistant: { commentary: false, final: true },
      tool_call: { exec_command: false, apply_patch: true, web_search: true, generic: true },
      reasoning_hidden: false,
      compacted: true,
      turn_separator: true,
      unknown: true,
    },
  },
};
