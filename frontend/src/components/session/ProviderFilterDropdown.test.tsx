import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProviderFilterDropdown from './ProviderFilterDropdown';
import { getColorValue, type FilterChip, type FilterChipGroup } from './filterChips';

function makeGroup(overrides: Partial<FilterChipGroup> = {}): {
  group: FilterChipGroup;
  onToggleParent: ReturnType<typeof vi.fn>;
  onToggleSub: ReturnType<typeof vi.fn>;
} {
  const onToggleParent = vi.fn();
  const onToggleSub = vi.fn();
  const group: FilterChipGroup = {
    key: 'assistant',
    label: 'Assistant',
    total: 5,
    color: getColorValue('blue'),
    expandNoun: 'assistant subcategories',
    toggleAllLabel: 'Toggle all assistant messages',
    onToggleParent,
    subItems: [
      { key: 'commentary', label: 'Commentary', count: 3, visible: true, color: getColorValue('blue'), onToggle: () => onToggleSub('commentary') },
      { key: 'final', label: 'Final', count: 2, visible: false, color: getColorValue('blue'), onToggle: () => onToggleSub('final') },
    ],
    ...overrides,
  };
  return { group, onToggleParent, onToggleSub };
}

function makeFlat(onToggle: () => void, count = 4, visible = true): FilterChip {
  return { key: 'user', label: 'User', count, visible, color: getColorValue('green'), onToggle };
}

async function open(getByRole: ReturnType<typeof render>['getByRole']) {
  const user = userEvent.setup();
  await user.click(getByRole('button', { name: 'Message Filters' }));
  return user;
}

describe('ProviderFilterDropdown', () => {
  it('renders flat chips and disables zero-count ones', async () => {
    const { group } = makeGroup();
    const { getByRole, getByText } = render(
      <ProviderFilterDropdown
        groups={[group]}
        flatItems={[makeFlat(vi.fn(), 0)]}
      />,
    );
    await open(getByRole);
    expect(getByText('User').closest('button')).toBeDisabled();
  });

  it('shows an indeterminate parent when subcategories are mixed', async () => {
    const { group } = makeGroup(); // commentary visible, final hidden → indeterminate
    const { getByRole } = render(<ProviderFilterDropdown groups={[group]} flatItems={[]} />);
    await open(getByRole);
    const parent = getByRole('button', { name: 'Toggle all assistant messages' });
    expect(parent.querySelector('[class*="indeterminate"]')).not.toBeNull();
  });

  it('expands a group to reveal subcategories and wires their toggle', async () => {
    const { group, onToggleSub } = makeGroup();
    const { getByRole, getByText } = render(<ProviderFilterDropdown groups={[group]} flatItems={[]} />);
    const user = await open(getByRole);
    await user.click(getByRole('button', { name: /Expand assistant subcategories/ }));
    await user.click(getByText('Final').closest('button')!);
    expect(onToggleSub).toHaveBeenCalledWith('final');
  });

  it('wires the parent toggle and flat-chip toggle', async () => {
    const { group, onToggleParent } = makeGroup();
    const flatToggle = vi.fn();
    const { getByRole, getByText } = render(
      <ProviderFilterDropdown groups={[group]} flatItems={[makeFlat(flatToggle)]} />,
    );
    const user = await open(getByRole);
    await user.click(getByRole('button', { name: 'Toggle all assistant messages' }));
    expect(onToggleParent).toHaveBeenCalledOnce();
    await user.click(getByText('User').closest('button')!);
    expect(flatToggle).toHaveBeenCalledOnce();
  });

  it('marks the filter button active when a category with rows is hidden', () => {
    const { group } = makeGroup(); // 'final' hidden with count 2 → active
    const { getByRole } = render(<ProviderFilterDropdown groups={[group]} flatItems={[]} />);
    expect(getByRole('button', { name: 'Message Filters' }).className).toMatch(/active/);
  });
});
