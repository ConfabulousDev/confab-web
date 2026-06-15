import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import ProviderFilterDropdown from './ProviderFilterDropdown';
import { getColorValue, type FilterChip, type FilterChipGroup } from './filterChips';

// A small interactive harness: track hidden keys locally and rebuild the
// declarative groups/flatItems each render so toggles are live in the story.
function Interactive({ initialHidden = [] }: { initialHidden?: string[] }) {
  const [hidden, setHidden] = useState<Set<string>>(new Set(initialHidden));
  const toggle = (key: string) =>
    setHidden((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const visible = (key: string) => !hidden.has(key);

  const sub = (parent: string, key: string, label: string, count: number, color: string): FilterChip => ({
    key,
    label,
    count,
    visible: visible(`${parent}.${key}`),
    color,
    onToggle: () => toggle(`${parent}.${key}`),
  });

  const groups: FilterChipGroup[] = [
    {
      key: 'assistant',
      label: 'Assistant',
      total: 21,
      color: getColorValue('blue'),
      expandNoun: 'assistant subcategories',
      toggleAllLabel: 'Toggle all assistant messages',
      onToggleParent: () => toggle('assistant'),
      subItems: [
        sub('assistant', 'commentary', 'Commentary', 9, getColorValue('blue')),
        sub('assistant', 'final', 'Final', 12, getColorValue('blue')),
      ],
    },
  ];

  const flat = (key: string, label: string, count: number, color: string): FilterChip => ({
    key,
    label,
    count,
    visible: visible(key),
    color,
    onToggle: () => toggle(key),
  });

  const flatItems: FilterChip[] = [
    flat('user', 'User', 12, getColorValue('green')),
    flat('compacted', 'Compacted', 1, getColorValue('cyan')),
    flat('unknown', 'Unknown', 0, getColorValue('default')), // zero-count → disabled
  ];

  return (
    <div style={{ display: 'flex', justifyContent: 'flex-end', padding: 24 }}>
      <ProviderFilterDropdown groups={groups} flatItems={flatItems} />
    </div>
  );
}

const meta: Meta<typeof Interactive> = {
  title: 'Session/ProviderFilterDropdown',
  component: Interactive,
};
export default meta;

type Story = StoryObj<typeof Interactive>;

export const Default: Story = {};

export const SomeHidden: Story = {
  args: { initialHidden: ['assistant.final', 'user'] },
};
