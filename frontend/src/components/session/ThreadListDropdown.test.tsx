// we3k D10 / jgk8: generalized thread list dropdown. Backs "All subagents (n)"
// (Main first + footer), the breadcrumb's sibling segments and "Launched here".

import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TranscriptThreadRef } from '@/providers/types';
import ThreadListDropdown from './ThreadListDropdown';

function thread(id: string, label: string, extra: Partial<TranscriptThreadRef> = {}): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId: null, launchTargetId: `r-${id}`, ...extra };
}

const threads: TranscriptThreadRef[] = [
  thread('a1', 'Explore the codebase', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('a2', 'Write tests', { status: 'completed', durationMs: 90_000 }),
  thread('a3', 'Simplify diff', { status: 'failed' }),
];

type Props = React.ComponentProps<typeof ThreadListDropdown>;

function renderDropdown(props: Partial<Props> = {}) {
  const onSelect = vi.fn();
  render(
    <ThreadListDropdown
      threads={threads}
      activeThreadId={null}
      onSelect={onSelect}
      label="All subagents (3)"
      listLabel="All subagents"
      includeMain
      {...props}
    />,
  );
  return { onSelect, button: screen.getByRole('button', { name: props.label ?? 'All subagents (3)' }) };
}

describe('ThreadListDropdown / trigger and rows', () => {
  it('is a collapsed listbox trigger named by its label', () => {
    const { button } = renderDropdown();
    expect(button).toHaveAttribute('aria-haspopup', 'listbox');
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('lists Main first when includeMain, then every thread with label, type, model, status and duration', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown();
    await user.click(button);
    expect(button).toHaveAttribute('aria-expanded', 'true');

    const options = within(screen.getByRole('listbox', { name: 'All subagents' })).getAllByRole('option');
    expect(options).toHaveLength(4);
    expect(options[0]).toHaveTextContent('Main');
    expect(options[1]).toHaveTextContent('Explore the codebase');
    expect(options[1]).toHaveTextContent('Explore');
    expect(options[1]).toHaveTextContent('claude-opus-5');
    expect(options[1]).toHaveTextContent('running');
    expect(options[2]).toHaveTextContent('1m 30s');
  });

  it('lists only the given threads without includeMain, and names the list by listLabel', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown({ includeMain: false, label: 'Launched here (3)', listLabel: 'Launched by Implement 0e6y' });
    await user.click(button);
    const list = screen.getByRole('listbox', { name: 'Launched by Implement 0e6y' });
    expect(within(list).getAllByRole('option').map((o) => o.textContent)).toEqual([
      expect.stringContaining('Explore the codebase'),
      expect.stringContaining('Write tests'),
      expect.stringContaining('Simplify diff'),
    ]);
  });

  it('renders custom trigger content while keeping the label as the accessible name', () => {
    renderDropdown({ label: 'Judge arrays r1', buttonContent: <span>Judge arr…</span> });
    expect(screen.getByRole('button', { name: 'Judge arrays r1' })).toHaveTextContent('Judge arr…');
  });

  it('shows "launched N" on rows whose children are known, and nothing for zero', async () => {
    const user = userEvent.setup();
    const childCountOf = (id: string) => ({ a1: 43, a2: 1 })[id] ?? 0;
    const { button } = renderDropdown({ childCountOf });
    await user.click(button);
    const options = screen.getAllByRole('option');
    expect(options[1]).toHaveTextContent('launched 43');
    expect(options[2]).toHaveTextContent('launched 1');
    expect(options[3]).not.toHaveTextContent(/launched/);
    expect(options[0]).not.toHaveTextContent(/launched/);
  });

  it('marks the active thread option as selected', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown({ activeThreadId: 'a2' });
    await user.click(button);
    const options = screen.getAllByRole('option');
    expect(options[2]).toHaveAttribute('aria-selected', 'true');
    expect(options[0]).toHaveAttribute('aria-selected', 'false');
  });

  it('checks nothing when the active thread is not in the list', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown({ includeMain: false, activeThreadId: 'elsewhere' });
    await user.click(button);
    for (const option of screen.getAllByRole('option')) expect(option).toHaveAttribute('aria-selected', 'false');
  });
});

describe('ThreadListDropdown / footer', () => {
  it('renders a muted footer below the list that is not an option', async () => {
    const user = userEvent.setup();
    const note = '347 more launched by subagents. Open one to see the ones it launched.';
    const { button } = renderDropdown({ footer: note });
    await user.click(button);
    const footer = screen.getByText(note);
    expect(screen.getByRole('listbox')).not.toContainElement(footer);
    expect(screen.getAllByRole('option')).toHaveLength(4);
  });

  it('keeps the footer when the filter matches nothing', async () => {
    const user = userEvent.setup();
    const many = Array.from({ length: 9 }, (_, i) => thread(`t${i}`, `Task ${i}`));
    const { button } = renderDropdown({ threads: many, footer: 'more below' });
    await user.click(button);
    await user.type(screen.getByRole('searchbox'), 'zzz');
    expect(screen.getByText('No subagents match')).toBeInTheDocument();
    expect(screen.getByText('more below')).toBeInTheDocument();
  });

  it('renders no footer by default', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown();
    await user.click(button);
    expect(screen.queryByText(/more launched/)).not.toBeInTheDocument();
  });
});

describe('ThreadListDropdown / selection, filter and keyboard', () => {
  it('selecting a row switches thread and closes the list', async () => {
    const user = userEvent.setup();
    const { button, onSelect } = renderDropdown();
    await user.click(button);
    await user.click(screen.getAllByRole('option')[2]!);
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

  it('shows no filter input for 8 or fewer threads', async () => {
    const user = userEvent.setup();
    const eight = Array.from({ length: 8 }, (_, i) => thread(`t${i}`, `Task ${i}`));
    const { button } = renderDropdown({ threads: eight });
    await user.click(button);
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument();
  });

  it('shows a focused filter for more than 8 threads that matches label, type or model', async () => {
    const user = userEvent.setup();
    const many = [
      ...Array.from({ length: 8 }, (_, i) => thread(`t${i}`, `Task ${i}`)),
      thread('h1', 'Summarize logs', { model: 'claude-haiku-4-5' }),
      thread('e1', 'Look around', { subtitle: 'Explore' }),
    ];
    const { button } = renderDropdown({ threads: many, includeMain: false });
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

  it('opens with the active row highlighted', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown({ includeMain: false, activeThreadId: 'a2' });
    await user.click(button);
    expect(screen.getAllByRole('option')[1]).toHaveAttribute('data-highlighted', 'true');
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

  it('ArrowUp from the first row wraps to the last', async () => {
    const user = userEvent.setup();
    const { button } = renderDropdown({ includeMain: false });
    await user.click(button);
    await user.keyboard('{ArrowUp}');
    expect(screen.getAllByRole('option')[2]).toHaveAttribute('data-highlighted', 'true');
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
