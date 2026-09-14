// we3k D10: "All subagents (n)" dropdown — every thread in strip order with
// details, a filter for long lists, keyboard selection.

import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TranscriptThreadRef } from '@/providers/types';
import AllThreadsDropdown from './AllThreadsDropdown';

function thread(id: string, label: string, extra: Partial<TranscriptThreadRef> = {}): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId: null, launchTargetId: `r-${id}`, ...extra };
}

const threads: TranscriptThreadRef[] = [
  thread('a1', 'Explore the codebase', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('c1', 'Read session store', { status: 'completed', parentThreadId: 'a1' }),
  thread('a2', 'Write tests', { status: 'completed', durationMs: 90_000 }),
];

function renderDropdown(props: Partial<React.ComponentProps<typeof AllThreadsDropdown>> = {}) {
  const onSelect = vi.fn();
  render(<AllThreadsDropdown threads={threads} activeThreadId={null} onSelect={onSelect} {...props} />);
  return { onSelect, button: screen.getByRole('button', { name: /^All subagents/ }) };
}

describe('AllThreadsDropdown', () => {
  it('is a collapsed listbox trigger labeled with the subagent count', () => {
    const { button } = renderDropdown();
    expect(button).toHaveAccessibleName('All subagents (3)');
    expect(button).toHaveAttribute('aria-haspopup', 'listbox');
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('lists Main first, then every thread with full label, type, model and duration', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown();
    await user.click(button);
    expect(button).toHaveAttribute('aria-expanded', 'true');

    const options = within(screen.getByRole('listbox')).getAllByRole('option');
    expect(options).toHaveLength(4);
    expect(options[0]).toHaveTextContent('Main');
    expect(options[1]).toHaveTextContent('Explore the codebase');
    expect(options[1]).toHaveTextContent('Explore');
    expect(options[1]).toHaveTextContent('claude-opus-5');
    expect(options[1]).toHaveTextContent('running');
    expect(options[2]).toHaveTextContent('Launched by Explore the codebase');
    expect(options[3]).toHaveTextContent('1m 30s');
  });

  it('marks the active thread option as selected', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown({ activeThreadId: 'a2' });
    await user.click(button);
    const options = screen.getAllByRole('option');
    expect(options[3]).toHaveAttribute('aria-selected', 'true');
    expect(options[0]).toHaveAttribute('aria-selected', 'false');
  });

  it('selecting a row switches thread and closes the list', async () => {
    const user = userEvent.setup();
    const { button, onSelect } = renderDropdown();
    await user.click(button);
    await user.click(screen.getAllByRole('option')[3]!);
    expect(onSelect).toHaveBeenCalledWith('a2');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('selecting Main passes null', async () => {
    const user = userEvent.setup();
    const { button, onSelect } = renderDropdown({ activeThreadId: 'a1' });
    await user.click(button);
    await user.click(screen.getAllByRole('option')[0]!);
    expect(onSelect).toHaveBeenCalledWith(null);
  });

  it('shows no filter input for 8 or fewer subagents', async () => {
    const user = userEvent.setup();
    const eight = Array.from({ length: 8 }, (_, i) => thread(`t${i}`, `Task ${i}`));
    const { button } = renderDropdown({ threads: eight });
    await user.click(button);
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument();
  });

  it('shows a focused filter for more than 8 subagents that matches label, type or model', async () => {
    const user = userEvent.setup();
    const many = [
      ...Array.from({ length: 8 }, (_, i) => thread(`t${i}`, `Task ${i}`)),
      thread('h1', 'Summarize logs', { model: 'claude-haiku-4-5' }),
      thread('e1', 'Look around', { subtitle: 'Explore' }),
    ];
    const { button } = renderDropdown({ threads: many });
    await user.click(button);
    const filter = screen.getByRole('searchbox', { name: 'Filter subagents' });
    expect(filter).toHaveFocus();

    await user.type(filter, 'HAIKU');
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([expect.stringContaining('Summarize logs')]);

    await user.clear(filter);
    await user.type(filter, 'explore');
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([expect.stringContaining('Look around')]);

    await user.clear(filter);
    await user.type(filter, 'nothing matches this');
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    expect(screen.getByText('No subagents match')).toBeInTheDocument();
  });

  it('ArrowDown then Enter selects the highlighted row', async () => {
    const user = userEvent.setup();
    const { button, onSelect } = renderDropdown();
    await user.click(button);
    await user.keyboard('{ArrowDown}');
    expect(screen.getAllByRole('option')[1]).toHaveAttribute('data-highlighted', 'true');
    await user.keyboard('{Enter}');
    expect(onSelect).toHaveBeenCalledWith('a1');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('Escape closes the list and returns focus to the button', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown();
    await user.click(button);
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(button).toHaveFocus();
  });
});
