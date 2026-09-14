// we3k: thread switcher under the Transcript tab — pinned Main, status chips in
// a scroller without native scrollbars, overflow fades + step buttons, the
// active chip's menu (Go to parent / Copy link), and manual-activation tabs.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TranscriptThreadRef } from '@/providers/types';
import TranscriptThreadTabs from './TranscriptThreadTabs';

function thread(id: string, label: string, extra: Partial<TranscriptThreadRef> = {}): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label, parentThreadId: null, launchTargetId: `r-${id}`, ...extra };
}

const threads: TranscriptThreadRef[] = [
  thread('a1', 'Explore the codebase', { status: 'running', subtitle: 'Explore', model: 'claude-opus-5' }),
  thread('a2', 'Write tests', { status: 'completed', durationMs: 90_000 }),
  thread('a3', 'Simplify diff', { status: 'failed' }),
];

type Props = React.ComponentProps<typeof TranscriptThreadTabs>;

function renderStrip(props: Partial<Props> = {}) {
  const onSelect = vi.fn();
  const utils = render(
    <TranscriptThreadTabs threads={threads} activeThreadId={null} sessionId="s1" onSelect={onSelect} {...props} />,
  );
  return { onSelect, ...utils };
}

function tab(name: string | RegExp) {
  return screen.getByRole('tab', { name });
}

function mockOverflow(el: HTMLElement, { scrollWidth, clientWidth, scrollLeft }: Record<string, number>) {
  Object.defineProperty(el, 'scrollWidth', { configurable: true, value: scrollWidth });
  Object.defineProperty(el, 'clientWidth', { configurable: true, value: clientWidth });
  Object.defineProperty(el, 'scrollLeft', { configurable: true, writable: true, value: scrollLeft });
}

describe('TranscriptThreadTabs / layout', () => {
  it('renders a tablist with Main first, then subagent chips in order', () => {
    renderStrip();
    expect(screen.getByRole('tablist')).toBeInTheDocument();
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual([
      'Main',
      'Explore the codebase',
      'Write tests',
      'Simplify diff',
    ]);
  });

  it('keeps Main outside the horizontal scroller so it never scrolls away', () => {
    renderStrip();
    const scroller = screen.getByTestId('thread-scroller');
    expect(scroller).not.toContainElement(tab('Main'));
    expect(scroller).toContainElement(tab(/^Explore the codebase/));
  });

  it('names each subagent chip with its status in text, not color alone', () => {
    renderStrip({ threads: [...threads, thread('a4', 'Deep linked'), thread('a5', 'Cancelled', { status: 'stopped' })] });
    expect(tab('Explore the codebase, running')).toBeInTheDocument();
    expect(tab('Write tests, completed')).toBeInTheDocument();
    expect(tab('Simplify diff, failed')).toBeInTheDocument();
    expect(tab('Cancelled, stopped')).toBeInTheDocument();
    expect(tab('Deep linked, status unknown')).toBeInTheDocument();
  });

  it('renders a distinct status glyph per chip', () => {
    renderStrip({ threads: [...threads, thread('a4', 'Deep linked')] });
    const glyphStatus = (name: RegExp) => tab(name).querySelector('[data-status]')?.getAttribute('data-status');
    expect(glyphStatus(/^Explore the codebase/)).toBe('running');
    expect(glyphStatus(/^Write tests/)).toBe('completed');
    expect(glyphStatus(/^Simplify diff/)).toBe('failed');
    expect(glyphStatus(/^Deep linked/)).toBe('unknown');
  });

  it('does not use the native title attribute on chips', () => {
    renderStrip();
    for (const t of screen.getAllByRole('tab')) expect(t).not.toHaveAttribute('title');
  });

  it('marks the active chip with aria-selected', () => {
    renderStrip({ activeThreadId: 'a2' });
    expect(tab(/^Write tests/)).toHaveAttribute('aria-selected', 'true');
    expect(tab('Main')).toHaveAttribute('aria-selected', 'false');
  });

  it('marks Main active when no thread is selected', () => {
    renderStrip();
    expect(tab('Main')).toHaveAttribute('aria-selected', 'true');
  });

  it('shows the full label, type, model and status in a tooltip on focus', () => {
    renderStrip();
    act(() => {
      tab(/^Explore the codebase/).focus();
    });
    const tooltip = screen.getByRole('tooltip');
    expect(tooltip).toHaveTextContent('Explore the codebase');
    expect(tooltip).toHaveTextContent('Explore');
    expect(tooltip).toHaveTextContent('claude-opus-5');
    expect(tooltip).toHaveTextContent('running');
  });

  it("shows the duration in the tooltip when known, and names a nested agent's parent", () => {
    renderStrip({
      threads: [...threads, thread('c1', 'Read session store', { parentThreadId: 'a1', status: 'running' })],
    });
    act(() => {
      tab(/^Write tests/).focus();
    });
    expect(screen.getByRole('tooltip')).toHaveTextContent('1m 30s');
    act(() => {
      tab(/^Read session store/).focus();
    });
    expect(screen.getByRole('tooltip')).toHaveTextContent('Launched by Explore the codebase');
  });

  it('marks a nested chip with an indent and keeps its own label', () => {
    renderStrip({
      threads: [threads[0]!, thread('c1', 'Read session store', { parentThreadId: 'a1' }), threads[1]!],
    });
    const nested = tab(/^Read session store/);
    expect(nested.textContent).toBe('Read session store');
    expect(nested.querySelector('[data-nested]')).not.toBeNull();
    expect(tab(/^Write tests/).querySelector('[data-nested]')).toBeNull();
  });

  it('renders the All subagents button with the subagent count', () => {
    renderStrip();
    expect(screen.getByRole('button', { name: 'All subagents (3)' })).toBeInTheDocument();
  });
});

describe('TranscriptThreadTabs / selection and chip menu', () => {
  it('selects an inactive chip on click, and Main with null', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderStrip({ activeThreadId: 'a1' });
    await user.click(tab(/^Write tests/));
    await user.click(tab('Main'));
    expect(onSelect).toHaveBeenNthCalledWith(1, 'a2');
    expect(onSelect).toHaveBeenNthCalledWith(2, null);
  });

  it('opens the menu when the already-active subagent chip is clicked, without re-selecting', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderStrip({ activeThreadId: 'a1' });
    const active = tab(/^Explore the codebase/);
    expect(active).toHaveAttribute('aria-haspopup', 'menu');
    expect(active).toHaveAttribute('aria-expanded', 'false');

    await user.click(active);
    expect(screen.getByRole('menu')).toBeInTheDocument();
    expect(active).toHaveAttribute('aria-expanded', 'true');
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('opens no menu for the active Main chip and gives inactive chips no popup', async () => {
    const user = userEvent.setup();
    renderStrip();
    expect(tab('Main')).not.toHaveAttribute('aria-haspopup');
    expect(tab(/^Write tests/)).not.toHaveAttribute('aria-haspopup');
    await user.click(tab('Main'));
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('Go to parent (Main) returns to the launch row and closes the menu', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderStrip({ activeThreadId: 'a1' });
    await user.click(tab(/^Explore the codebase/));
    await user.click(screen.getByRole('menuitem', { name: 'Go to parent (Main)' }));
    expect(onSelect).toHaveBeenCalledWith(null, 'r-a1');
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it("Go to parent names a nested agent's parent subagent and passes its id", async () => {
    const user = userEvent.setup();
    const child = thread('c1', 'Read session store', { parentThreadId: 'a1', launchTargetId: 'rc' });
    const { onSelect } = renderStrip({ threads: [...threads, child], activeThreadId: 'c1' });
    await user.click(tab(/^Read session store/));
    await user.click(screen.getByRole('menuitem', { name: 'Go to parent (Explore the codebase)' }));
    expect(onSelect).toHaveBeenCalledWith('a1', 'rc');
  });

  it('hides Go to parent when the parent is unknown (deep link)', async () => {
    const user = userEvent.setup();
    const orphan = thread('zz', 'zz', { parentThreadId: undefined, launchTargetId: undefined });
    renderStrip({ threads: [...threads, orphan], activeThreadId: 'zz' });
    await user.click(tab(/^zz/));
    expect(screen.getAllByRole('menuitem').map((m) => m.textContent)).toEqual(['Copy link to this subagent']);
  });

  it('Copy link writes the subagent deep link and confirms', async () => {
    const user = userEvent.setup();
    renderStrip({ activeThreadId: 'a1' });
    await user.click(tab(/^Explore the codebase/));
    await user.click(screen.getByRole('menuitem', { name: 'Copy link to this subagent' }));
    await expect(navigator.clipboard.readText()).resolves.toBe(
      `${window.location.origin}/sessions/s1?tab=transcript&agent=a1`,
    );
    expect(await screen.findByRole('menuitem', { name: 'Copied' })).toBeInTheDocument();
  });

  it('Escape closes the chip menu and returns focus to the chip', async () => {
    const user = userEvent.setup();
    renderStrip({ activeThreadId: 'a1' });
    const active = tab(/^Explore the codebase/);
    await user.click(active);
    expect(screen.getByRole('menu')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    expect(active).toHaveFocus();
  });

  it('Enter on the focused active subagent chip opens its menu', async () => {
    const user = userEvent.setup();
    renderStrip({ activeThreadId: 'a2' });
    act(() => {
      tab(/^Write tests/).focus();
    });
    await user.keyboard('{Enter}');
    expect(screen.getByRole('menu')).toBeInTheDocument();
  });

  it('closes the chip menu when another thread becomes active', async () => {
    const user = userEvent.setup();
    const { rerender, onSelect } = renderStrip({ activeThreadId: 'a1' });
    await user.click(tab(/^Explore the codebase/));
    expect(screen.getByRole('menu')).toBeInTheDocument();
    rerender(<TranscriptThreadTabs threads={threads} activeThreadId="a2" sessionId="s1" onSelect={onSelect} />);
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });
});

describe('TranscriptThreadTabs / keyboard', () => {
  const originalScrollIntoView = Element.prototype.scrollIntoView;
  afterEach(() => {
    Element.prototype.scrollIntoView = originalScrollIntoView;
  });

  it('uses a roving tabindex: only the active chip is tabbable', () => {
    renderStrip({ activeThreadId: 'a2' });
    expect(tab(/^Write tests/)).toHaveAttribute('tabindex', '0');
    expect(tab('Main')).toHaveAttribute('tabindex', '-1');
    expect(tab(/^Explore the codebase/)).toHaveAttribute('tabindex', '-1');
  });

  it('arrow keys, Home and End move focus without selecting; Enter selects (manual activation)', async () => {
    const user = userEvent.setup();
    const { onSelect } = renderStrip();
    await user.tab();
    expect(tab('Main')).toHaveFocus();

    await user.keyboard('{ArrowRight}');
    expect(tab(/^Explore the codebase/)).toHaveFocus();
    expect(tab(/^Explore the codebase/)).toHaveAttribute('tabindex', '0');
    await user.keyboard('{ArrowRight}');
    expect(tab(/^Write tests/)).toHaveFocus();
    await user.keyboard('{End}');
    expect(tab(/^Simplify diff/)).toHaveFocus();
    await user.keyboard('{Home}');
    expect(tab('Main')).toHaveFocus();
    await user.keyboard('{ArrowLeft}');
    expect(tab(/^Simplify diff/)).toHaveFocus();
    expect(onSelect).not.toHaveBeenCalled();

    await user.keyboard('{Enter}');
    expect(onSelect).toHaveBeenCalledWith('a3');
  });

  it('scrolls the focused chip into view', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    const user = userEvent.setup();
    renderStrip();
    await user.tab();
    scrollIntoView.mockClear();
    await user.keyboard('{ArrowRight}');
    expect(scrollIntoView).toHaveBeenCalledWith(expect.objectContaining({ inline: 'nearest' }));
    expect(scrollIntoView.mock.contexts).toContain(tab(/^Explore the codebase/));
  });
});

describe('TranscriptThreadTabs / overflow', () => {
  // Step buttons are aria-hidden mouse affordances (buttons aren't allowed inside a
  // tablist), so a hidden element has no accessible name to query by role.
  function stepButton(direction: 'left' | 'right') {
    return document.querySelector<HTMLButtonElement>(`button[aria-label="Scroll subagents ${direction}"]`);
  }

  it('shows no edge fades or step buttons when every chip fits', () => {
    renderStrip();
    expect(stepButton('left')).not.toBeInTheDocument();
    expect(stepButton('right')).not.toBeInTheDocument();
    expect(screen.getByTestId('thread-scroller').className).not.toMatch(/fade/);
  });

  it('signals hidden chips with an edge fade and a step button on that side only', async () => {
    renderStrip();
    const scroller = screen.getByTestId('thread-scroller');

    mockOverflow(scroller, { scrollWidth: 1000, clientWidth: 400, scrollLeft: 0 });
    fireEvent.scroll(scroller);
    await waitFor(() => expect(stepButton('right')).toBeInTheDocument());
    expect(stepButton('left')).not.toBeInTheDocument();
    expect(scroller.className).toMatch(/fadeEnd/);

    scroller.scrollLeft = 300;
    fireEvent.scroll(scroller);
    await waitFor(() => expect(stepButton('left')).toBeInTheDocument());
    expect(stepButton('right')).toBeInTheDocument();
    expect(scroller.className).toMatch(/fadeBoth/);

    scroller.scrollLeft = 600;
    fireEvent.scroll(scroller);
    await waitFor(() => expect(stepButton('right')).not.toBeInTheDocument());
    expect(stepButton('left')).toBeInTheDocument();
    expect(scroller.className).toMatch(/fadeStart/);
  });

  it('step buttons scroll by most of the visible width', async () => {
    renderStrip();
    const scroller = screen.getByTestId('thread-scroller');
    const scrollBy = vi.fn();
    scroller.scrollBy = scrollBy;
    mockOverflow(scroller, { scrollWidth: 1000, clientWidth: 400, scrollLeft: 300 });
    fireEvent.scroll(scroller);
    await waitFor(() => expect(stepButton('right')).toBeInTheDocument());

    fireEvent.click(stepButton('right')!);
    expect(scrollBy).toHaveBeenLastCalledWith(expect.objectContaining({ left: 320 }));
    fireEvent.click(stepButton('left')!);
    expect(scrollBy).toHaveBeenLastCalledWith(expect.objectContaining({ left: -320 }));
  });

  it('a vertical wheel over the strip scrolls it sideways', () => {
    renderStrip();
    const scroller = screen.getByTestId('thread-scroller');
    mockOverflow(scroller, { scrollWidth: 1000, clientWidth: 400, scrollLeft: 0 });
    fireEvent.wheel(scroller, { deltaY: 120, deltaX: 0 });
    expect(scroller.scrollLeft).toBe(120);
  });

  it('leaves a wheel alone when the strip does not overflow', () => {
    renderStrip();
    const scroller = screen.getByTestId('thread-scroller');
    mockOverflow(scroller, { scrollWidth: 400, clientWidth: 400, scrollLeft: 0 });
    fireEvent.wheel(scroller, { deltaY: 120, deltaX: 0 });
    expect(scroller.scrollLeft).toBe(0);
  });
});
