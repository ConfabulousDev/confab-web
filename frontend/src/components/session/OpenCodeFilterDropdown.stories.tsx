import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import OpenCodeFilterDropdown from './OpenCodeFilterDropdown';
import {
  DEFAULT_OPENCODE_FILTER_STATE,
  type OpenCodeCategory,
  type OpenCodeFilterState,
} from './opencodeCategories';

const meta: Meta<typeof OpenCodeFilterDropdown> = {
  title: 'Session/OpenCodeFilterDropdown',
  component: OpenCodeFilterDropdown,
  parameters: { layout: 'centered' },
};

export default meta;
type Story = StoryObj<typeof OpenCodeFilterDropdown>;

function Interactive({ counts }: { counts: { user: number; assistant: number; tool: number } }) {
  const [state, setState] = useState<OpenCodeFilterState>({ ...DEFAULT_OPENCODE_FILTER_STATE });
  return (
    <OpenCodeFilterDropdown
      counts={counts}
      filterState={state}
      onToggleCategory={(c: OpenCodeCategory) => setState((p) => ({ ...p, [c]: !p[c] }))}
    />
  );
}

export const Default: Story = {
  render: () => <Interactive counts={{ user: 4, assistant: 6, tool: 9 }} />,
};

export const SomeEmptyCategories: Story = {
  render: () => <Interactive counts={{ user: 2, assistant: 3, tool: 0 }} />,
};
