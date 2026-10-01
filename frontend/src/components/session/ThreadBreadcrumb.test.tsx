// jgk8: per-level subagent path under the thread strip. Main and the depth-1
// agent are links; each deeper level is a dropdown of its siblings; a trailing
// "Launched here (n)" lists the open agent's children; nested agents get a
// copy-link button.

import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TranscriptThreadRef } from '@/providers/types';
import ThreadBreadcrumb from './ThreadBreadcrumb';

function thread(id: string, label: string, parentThreadId: string | null | undefined): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId, launchTargetId: `launch-${id}`, status: 'completed' };
}

const impl = thread('impl', 'Implement 0e6y verification step', null);
const judgeArrays = thread('j1', 'Judge arrays r1', 'impl');
const judgeArrays2 = thread('j2', 'Judge arrays r2', 'impl');
const simplify = thread('j3', 'Simplify 0e6y Go changes', 'impl');
const grandchild = thread('g1', 'Re-run the flaky judge', 'j1');

const children: Record<string, TranscriptThreadRef[]> = {
  impl: [judgeArrays, judgeArrays2, simplify],
  j1: [grandchild],
};
const childrenOf = (id: string) => children[id] ?? [];

type Props = React.ComponentProps<typeof ThreadBreadcrumb>;

function renderCrumbs(props: Partial<Props> & Pick<Props, 'active' | 'path'>) {
  const onSelect = vi.fn();
  render(<ThreadBreadcrumb sessionId="s1" childrenOf={childrenOf} onSelect={onSelect} {...props} />);
  return { onSelect };
}

function nav() {
  return screen.queryByRole('navigation', { name: 'Subagent path' });
}

describe('ThreadBreadcrumb / when it shows', () => {
  it('renders nothing for a depth-1 agent that launched no subagents', () => {
    const lone = thread('a2', 'Write tests', null);
    renderCrumbs({ active: lone, path: [lone] });
    expect(nav()).not.toBeInTheDocument();
  });

  it('renders nothing for a cold deep link with no known children', () => {
    const cold = thread('zz', 'zz', undefined);
    renderCrumbs({ active: cold, path: undefined });
    expect(nav()).not.toBeInTheDocument();
  });

  it('shows Main, the current depth-1 agent and "Launched here (n)" for a depth-1 agent with children', () => {
    renderCrumbs({ active: impl, path: [impl] });
    const crumbs = within(nav()!);
    expect(crumbs.getByRole('button', { name: 'Main' })).toBeInTheDocument();
    expect(crumbs.getByText('Implement 0e6y verification step').closest('[aria-current]')).toHaveAttribute(
      'aria-current',
      'location',
    );
    expect(crumbs.queryByRole('button', { name: 'Implement 0e6y verification step' })).not.toBeInTheDocument();
    expect(crumbs.getByRole('button', { name: 'Launched here (3)' })).toBeInTheDocument();
    expect(crumbs.queryByRole('button', { name: 'Copy link to this subagent' })).not.toBeInTheDocument();
  });

  it('shows only "Launched here" for a cold deep link whose children are known (no ancestor crumbs)', () => {
    const cold = { ...judgeArrays, parentThreadId: undefined };
    renderCrumbs({ active: cold, path: undefined });
    const crumbs = within(nav()!);
    expect(crumbs.getAllByRole('button').map((b) => b.getAttribute('aria-label') ?? b.textContent)).toEqual([
      'Launched here (1)',
    ]);
  });
});

describe('ThreadBreadcrumb / Launched here', () => {
  it("lists the open agent's children, none checked, and opens the picked one", async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: impl, path: [impl] });
    await user.click(screen.getByRole('button', { name: 'Launched here (3)' }));
    const list = screen.getByRole('listbox', { name: 'Launched by Implement 0e6y verification step' });
    const options = within(list).getAllByRole('option');
    expect(options.map((o) => o.textContent)).toEqual([
      expect.stringContaining('Judge arrays r1'),
      expect.stringContaining('Judge arrays r2'),
      expect.stringContaining('Simplify 0e6y Go changes'),
    ]);
    for (const option of options) expect(option).toHaveAttribute('aria-selected', 'false');
    await user.click(options[1]!);
    expect(onSelect).toHaveBeenCalledWith('j2');
  });

  it('shows "launched N" on rows whose own children are known', async () => {
    const user = userEvent.setup();
    renderCrumbs({ active: impl, path: [impl] });
    await user.click(screen.getByRole('button', { name: 'Launched here (3)' }));
    const options = screen.getAllByRole('option');
    expect(options[0]).toHaveTextContent('launched 1');
    expect(options[1]).not.toHaveTextContent(/launched/);
  });

  it('gets a filter once more than 8 children are known', async () => {
    const user = userEvent.setup();
    const many = Array.from({ length: 43 }, (_, i) => thread(`k${i}`, `Judge task ${i} r1`, 'impl'));
    renderCrumbs({ active: impl, path: [impl], childrenOf: (id) => (id === 'impl' ? many : []) });
    await user.click(screen.getByRole('button', { name: 'Launched here (43)' }));
    expect(screen.getByRole('searchbox', { name: 'Filter subagents' })).toHaveFocus();
  });
});

describe('ThreadBreadcrumb / nested path', () => {
  it('at depth 2: Main and depth-1 links land on the launch row of the child you came from', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: judgeArrays, path: [impl, judgeArrays] });
    await user.click(screen.getByRole('button', { name: 'Main' }));
    expect(onSelect).toHaveBeenLastCalledWith(null, 'launch-impl');
    await user.click(screen.getByRole('button', { name: 'Implement 0e6y verification step' }));
    expect(onSelect).toHaveBeenLastCalledWith('impl', 'launch-j1');
  });

  it('at depth 2: the current segment is a dropdown of its siblings with the current one checked', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: judgeArrays2, path: [impl, judgeArrays2] });
    const segment = screen.getByRole('button', { name: 'Judge arrays r2' });
    expect(segment.closest('[aria-current]')).toHaveAttribute('aria-current', 'location');
    await user.click(segment);
    const options = within(screen.getByRole('listbox', { name: 'Launched by Implement 0e6y verification step' })).getAllByRole(
      'option',
    );
    expect(options).toHaveLength(3);
    expect(options[1]).toHaveAttribute('aria-selected', 'true');
    await user.click(options[2]!);
    expect(onSelect).toHaveBeenCalledWith('j3');
  });

  it('picking the already-open agent in its own dropdown does nothing', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: judgeArrays2, path: [impl, judgeArrays2] });
    await user.click(screen.getByRole('button', { name: 'Judge arrays r2' }));
    await user.click(screen.getAllByRole('option')[1]!);
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('at depth 3: two dropdown segments, and picking the depth-2 ancestor lands on the launch row', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: grandchild, path: [impl, judgeArrays, grandchild] });
    const dropdowns = within(nav()!)
      .getAllByRole('button')
      .filter((b) => b.getAttribute('aria-haspopup') === 'listbox');
    expect(dropdowns.map((b) => b.getAttribute('aria-label'))).toEqual(['Judge arrays r1', 'Re-run the flaky judge']);

    await user.click(dropdowns[0]!);
    const options = screen.getAllByRole('option');
    expect(options[0]).toHaveAttribute('aria-selected', 'true');
    await user.click(options[0]!);
    expect(onSelect).toHaveBeenCalledWith('j1', 'launch-g1');
  });

  it('falls back to just the current agent when its siblings are not known', async () => {
    const user = userEvent.setup();
    renderCrumbs({ active: judgeArrays, path: [impl, judgeArrays], childrenOf: () => [] });
    await user.click(screen.getByRole('button', { name: 'Judge arrays r1' }));
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([expect.stringContaining('Judge arrays r1')]);
  });

  it('adds "Launched here" after a nested agent that launched its own', () => {
    renderCrumbs({ active: judgeArrays, path: [impl, judgeArrays] });
    expect(screen.getByRole('button', { name: 'Launched here (1)' })).toBeInTheDocument();
  });
});

describe('ThreadBreadcrumb / copy link', () => {
  it('copies the nested agent deep link and confirms', async () => {
    const user = userEvent.setup();
    renderCrumbs({ active: judgeArrays, path: [impl, judgeArrays] });
    await user.click(screen.getByRole('button', { name: 'Copy link to this subagent' }));
    await expect(navigator.clipboard.readText()).resolves.toBe(
      `${window.location.origin}/sessions/s1?tab=transcript&agent=j1`,
    );
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument();
  });
});
