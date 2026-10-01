// jgk8 (D9 review: trim + inline): nested-subagent path row. Only at depth ≥ 2;
// starts at depth 2 (Main and the depth-1 agent are in the strip); each level is
// a dropdown of its siblings; a trailing "Launched here (n)" lists the open
// agent's children; copy link at the end.

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
const grandchild = { ...thread('g1', 'Re-run the flaky judge', 'j1'), status: 'running' as const };

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

function buttonNames() {
  return within(nav()!)
    .getAllByRole('button')
    .map((b) => b.getAttribute('aria-label') ?? b.textContent);
}

describe('ThreadBreadcrumb / when it shows', () => {
  it('renders nothing for a depth-1 agent, even one with children (the strip has its Launched here)', () => {
    renderCrumbs({ active: impl, path: [impl] });
    expect(nav()).not.toBeInTheDocument();
  });

  it('renders nothing for a cold deep link with no known children', () => {
    const cold = thread('zz', 'zz', undefined);
    renderCrumbs({ active: cold, path: undefined });
    expect(nav()).not.toBeInTheDocument();
  });

  it('shows only "Launched here" and copy link for a cold deep link whose children are known', () => {
    const cold = { ...judgeArrays, parentThreadId: undefined };
    renderCrumbs({ active: cold, path: undefined });
    expect(buttonNames()).toEqual(['Launched here (1)', 'Copy link to this subagent']);
  });

  it('starts at depth 2: no Main and no depth-1 crumb', () => {
    renderCrumbs({ active: judgeArrays2, path: [impl, judgeArrays2] });
    expect(buttonNames()).toEqual(['Judge arrays r2', 'Copy link to this subagent']);
    expect(within(nav()!).queryByText('Main')).not.toBeInTheDocument();
    expect(within(nav()!).queryByText('Implement 0e6y verification step')).not.toBeInTheDocument();
  });
});

describe('ThreadBreadcrumb / segments', () => {
  it("shows each segment's status glyph and its level's sibling count", () => {
    renderCrumbs({ active: grandchild, path: [impl, judgeArrays, grandchild] });
    const [judgeSegment, grandSegment] = within(nav()!)
      .getAllByRole('button')
      .filter((b) => b.getAttribute('aria-haspopup') === 'listbox');
    expect(judgeSegment).toHaveTextContent('Judge arrays r1(3)');
    expect(judgeSegment!.querySelector('[data-status]')).toHaveAttribute('data-status', 'completed');
    expect(grandSegment).toHaveTextContent('Re-run the flaky judge(1)');
    expect(grandSegment!.querySelector('[data-status]')).toHaveAttribute('data-status', 'running');
  });

  it('marks the open agent segment as the current location', () => {
    renderCrumbs({ active: judgeArrays2, path: [impl, judgeArrays2] });
    expect(screen.getByRole('button', { name: 'Judge arrays r2' }).closest('[aria-current]')).toHaveAttribute(
      'aria-current',
      'location',
    );
  });

  it('a segment is a dropdown of its siblings with the current one checked; picking one switches', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: judgeArrays2, path: [impl, judgeArrays2] });
    await user.click(screen.getByRole('button', { name: 'Judge arrays r2' }));
    const options = within(
      screen.getByRole('listbox', { name: 'Launched by Implement 0e6y verification step' }),
    ).getAllByRole('option');
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

  it('at depth 3, picking the depth-2 ancestor lands on the launch row of the agent you came from', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: grandchild, path: [impl, judgeArrays, grandchild] });
    await user.click(screen.getByRole('button', { name: 'Judge arrays r1' }));
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

  it('gets a filter once a level has more than 8 siblings', async () => {
    const user = userEvent.setup();
    const many = [judgeArrays, ...Array.from({ length: 42 }, (_, i) => thread(`k${i}`, `Judge task ${i} r1`, 'impl'))];
    renderCrumbs({ active: judgeArrays, path: [impl, judgeArrays], childrenOf: (id) => (id === 'impl' ? many : []) });
    expect(screen.getByRole('button', { name: 'Judge arrays r1' })).toHaveTextContent('(43)');
    await user.click(screen.getByRole('button', { name: 'Judge arrays r1' }));
    expect(screen.getByRole('searchbox', { name: 'Filter subagents' })).toHaveFocus();
  });
});

describe('ThreadBreadcrumb / Launched here', () => {
  it('appears after a nested agent that launched its own, listing them unchecked', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCrumbs({ active: judgeArrays, path: [impl, judgeArrays] });
    expect(buttonNames()).toEqual(['Judge arrays r1', 'Launched here (1)', 'Copy link to this subagent']);
    await user.click(screen.getByRole('button', { name: 'Launched here (1)' }));
    const option = within(screen.getByRole('listbox', { name: 'Launched by Judge arrays r1' })).getByRole('option');
    expect(option).toHaveAttribute('aria-selected', 'false');
    await user.click(option);
    expect(onSelect).toHaveBeenCalledWith('g1');
  });

  it('shows "launched N" on sibling rows whose own children are known', async () => {
    const user = userEvent.setup();
    renderCrumbs({ active: judgeArrays2, path: [impl, judgeArrays2] });
    await user.click(screen.getByRole('button', { name: 'Judge arrays r2' }));
    const options = screen.getAllByRole('option');
    expect(options[0]).toHaveTextContent('launched 1');
    expect(options[1]).not.toHaveTextContent(/launched/);
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
