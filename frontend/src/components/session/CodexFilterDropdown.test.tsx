import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import CodexFilterDropdown from './CodexFilterDropdown';
import type { CodexHierarchicalCounts, CodexFilterState } from './codexCategories';

function makeCounts(overrides: Partial<CodexHierarchicalCounts> = {}): CodexHierarchicalCounts {
  return {
    user: 3,
    assistant: { total: 4, commentary: 2, final: 2 },
    tool_call: { total: 5, exec_command: 2, apply_patch: 2, web_search: 0, generic: 1 },
    reasoning_hidden: 1,
    compacted: 0,
    turn_separator: 2,
    turn_aborted: 0,
    unknown: 0,
    ...overrides,
  };
}

function makeFilterState(overrides: Partial<CodexFilterState> = {}): CodexFilterState {
  return {
    user: true,
    assistant: { commentary: true, final: true },
    tool_call: { exec_command: true, apply_patch: true, web_search: true, generic: true },
    reasoning_hidden: false,
    compacted: true,
    turn_separator: true,
    turn_aborted: true,
    unknown: true,
    ...overrides,
  };
}

function renderDropdown(
  overrides: {
    counts?: Partial<CodexHierarchicalCounts>;
    filterState?: Partial<CodexFilterState>;
  } = {}
) {
  const handlers = {
    onToggleCategory: vi.fn(),
    onToggleAssistantSubcategory: vi.fn(),
    onToggleToolCallSubcategory: vi.fn(),
  };
  const utils = render(
    <CodexFilterDropdown
      counts={makeCounts(overrides.counts)}
      filterState={makeFilterState(overrides.filterState)}
      {...handlers}
    />
  );
  return { ...utils, ...handlers };
}

describe('CodexFilterDropdown', () => {
  it('does not render dropdown until the button is clicked', () => {
    const { queryByText } = renderDropdown();
    expect(queryByText('Message Filters')).toBeNull();
  });

  it('opens dropdown on button click', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    expect(getByText('Message Filters')).toBeInTheDocument();
  });

  it('disables a flat category whose count is 0', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    const compactedBtn = getByText('Compacted').closest('button');
    expect(compactedBtn).toBeDisabled();
  });

  it('renders parent rollup as indeterminate for tool_call when one sub is off', async () => {
    const user = userEvent.setup();
    const { getByRole } = renderDropdown({
      filterState: {
        tool_call: { exec_command: false, apply_patch: true, web_search: true, generic: true },
      },
    });
    await user.click(getByRole('button', { name: 'Message Filters' }));
    const toolCallBtn = getByRole('button', { name: 'Toggle all tool calls' });
    expect(toolCallBtn.querySelector('[class*="indeterminate"]')).not.toBeNull();
  });

  it('clicking a flat category invokes onToggleCategory', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText, onToggleCategory } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    await user.click(getByText('User').closest('button')!);
    expect(onToggleCategory).toHaveBeenCalledWith('user');
  });

  it('clicking an assistant subcategory invokes onToggleAssistantSubcategory', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText, onToggleAssistantSubcategory } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    await user.click(getByRole('button', { name: /Expand assistant subcategories/ }));
    await user.click(getByText('Final').closest('button')!);
    expect(onToggleAssistantSubcategory).toHaveBeenCalledWith('final');
  });

  it('clicking a tool_call subcategory invokes onToggleToolCallSubcategory', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText, onToggleToolCallSubcategory } = renderDropdown();
    await user.click(getByRole('button', { name: 'Message Filters' }));
    await user.click(getByRole('button', { name: /Expand tool call subcategories/ }));
    await user.click(getByText('Exec Command').closest('button')!);
    expect(onToggleToolCallSubcategory).toHaveBeenCalledWith('exec_command');
  });

  it('marks the button active when a non-empty category is hidden', () => {
    const { getByRole } = renderDropdown({
      filterState: { reasoning_hidden: false }, // count 1, hidden → active
    });
    const btn = getByRole('button', { name: 'Message Filters' });
    expect(btn.className).toMatch(/active/);
  });

  it('does not mark active when only zero-count categories are hidden', () => {
    const { getByRole } = renderDropdown({
      filterState: { reasoning_hidden: true, compacted: false }, // compacted count is 0
    });
    expect(getByRole('button', { name: 'Message Filters' }).className).not.toMatch(/\bactive\b/);
  });
});
