import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FilterDropdown from './FilterDropdown';
import type { HierarchicalCounts, FilterState } from './messageCategories';

function makeCounts(overrides: Partial<HierarchicalCounts> = {}): HierarchicalCounts {
  return {
    user: { total: 5, prompt: 3, 'tool-result': 1, skill: 1 },
    assistant: { total: 5, text: 3, 'tool-use': 1, thinking: 1 },
    attachment: {
      total: 0,
      hook: 0,
      'file-edit': 0,
      'queued-command': 0,
      'deferred-tools': 0,
      'mcp-instructions': 0,
    },
    system: 1,
    'file-history-snapshot': 0,
    summary: 0,
    'queue-operation': 0,
    'pr-link': 0,
    'away-summary': 0,
    unknown: 0,
    ...overrides,
  };
}

function makeFilterState(overrides: Partial<FilterState> = {}): FilterState {
  return {
    user: { prompt: true, 'tool-result': true, skill: true },
    assistant: { text: true, 'tool-use': true, thinking: true },
    attachment: {
      hook: false,
      'file-edit': false,
      'queued-command': false,
      'deferred-tools': false,
      'mcp-instructions': false,
    },
    system: true,
    'file-history-snapshot': true,
    summary: true,
    'queue-operation': true,
    'pr-link': true,
    'away-summary': true,
    unknown: true,
    ...overrides,
  };
}

function renderDropdown(
  overrides: {
    counts?: Partial<HierarchicalCounts>;
    filterState?: Partial<FilterState>;
  } = {}
) {
  const handlers = {
    onToggleCategory: vi.fn(),
    onToggleUserSubcategory: vi.fn(),
    onToggleAssistantSubcategory: vi.fn(),
    onToggleAttachmentSubcategory: vi.fn(),
  };
  const utils = render(
    <FilterDropdown
      counts={makeCounts(overrides.counts)}
      filterState={makeFilterState(overrides.filterState)}
      {...handlers}
    />
  );
  return { ...utils, ...handlers };
}

describe('FilterDropdown', () => {
  it('does not show dropdown until the filter button is clicked', () => {
    const { queryByText } = renderDropdown();
    expect(queryByText('Message Filters')).toBeNull();
  });

  it('opens dropdown on button click', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    expect(getByText('Message Filters')).toBeInTheDocument();
  });

  it('disables category buttons whose count is 0', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText } = renderDropdown({
      counts: { system: 0, 'pr-link': 0 },
    });
    await user.click(getByRole('button', { name: 'Message Filters' }));
    const sys = getByText('System').closest('button');
    expect(sys).toBeDisabled();
  });

  it('clicking a flat category invokes onToggleCategory with that key', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText, onToggleCategory } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    await user.click(getByText('System').closest('button')!);
    expect(onToggleCategory).toHaveBeenCalledWith('system');
  });

  it('clicking a user subcategory invokes onToggleUserSubcategory', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText, onToggleUserSubcategory } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    await user.click(getByRole('button', { name: /Expand user subcategories/ }));
    await user.click(getByText('Prompts').closest('button')!);
    expect(onToggleUserSubcategory).toHaveBeenCalledWith('prompt');
  });

  it('renders parent rollup as checked when all subs are visible', async () => {
    const user = userEvent.setup();
    const { getByRole } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    const userBtn = getByRole('button', { name: 'Toggle all user messages' });
    expect(userBtn.querySelector('[class*="checked"]')).not.toBeNull();
  });

  it('renders parent rollup as indeterminate when subs are mixed', async () => {
    const user = userEvent.setup();
    const { getByRole } = renderDropdown({
      filterState: { user: { prompt: true, 'tool-result': false, skill: true } },
    });
    await user.click(getByRole('button', { name: 'Message Filters' }));
    const userBtn = getByRole('button', { name: 'Toggle all user messages' });
    expect(userBtn.querySelector('[class*="indeterminate"]')).not.toBeNull();
  });

  it('renders parent rollup as unchecked when all subs are hidden', async () => {
    const user = userEvent.setup();
    const { getByRole } = renderDropdown({
      filterState: { user: { prompt: false, 'tool-result': false, skill: false } },
    });
    await user.click(getByRole('button', { name: 'Message Filters' }));
    const userBtn = getByRole('button', { name: 'Toggle all user messages' });
    expect(userBtn.querySelector('[class*="unchecked"]')).not.toBeNull();
  });

  it('marks the filter button as active when a non-empty category is hidden', () => {
    const { getByRole } = renderDropdown({
      filterState: { system: false },
    });
    const btn = getByRole('button', { name: 'Message Filters' });
    expect(btn.className).toMatch(/active/);
  });

  it('does not mark the filter button as active when no non-empty category is hidden', () => {
    const { getByRole } = renderDropdown();
    const btn = getByRole('button', { name: 'Message Filters' });
    expect(btn.className).not.toMatch(/\bactive\b/);
  });

  it('expand chevron toggles subcategory list visibility', async () => {
    const user = userEvent.setup();
    const { getByRole, queryByText } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));

    expect(queryByText('Prompts')).toBeNull();
    await user.click(getByRole('button', { name: /Expand user subcategories/ }));
    expect(queryByText('Prompts')).not.toBeNull();

    await user.click(getByRole('button', { name: /Collapse user subcategories/ }));
    expect(queryByText('Prompts')).toBeNull();
  });
});
