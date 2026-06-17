import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import CursorFilterDropdown from './CursorFilterDropdown';
import {
  DEFAULT_CURSOR_FILTER_STATE,
  type CursorCategory,
  type CursorFilterState,
} from './cursorCategories';

const meta: Meta<typeof CursorFilterDropdown> = {
  title: 'Session/CursorFilterDropdown',
  component: CursorFilterDropdown,
  parameters: { layout: 'centered' },
};

export default meta;
type Story = StoryObj<typeof CursorFilterDropdown>;

function Interactive({
  counts,
}: {
  counts: { user: number; assistant: number; tool: number };
}) {
  const [state, setState] = useState<CursorFilterState>({ ...DEFAULT_CURSOR_FILTER_STATE });
  return (
    <CursorFilterDropdown
      counts={counts}
      filterState={state}
      onToggleCategory={(c: CursorCategory) => setState((p) => ({ ...p, [c]: !p[c] }))}
    />
  );
}

export const Default: Story = {
  render: () => <Interactive counts={{ user: 4, assistant: 6, tool: 9 }} />,
};

export const SomeEmptyCategories: Story = {
  render: () => <Interactive counts={{ user: 2, assistant: 3, tool: 0 }} />,
};
