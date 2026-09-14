// we3k D5: shared themed tooltip (hover + keyboard focus), replacing native `title`.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Tooltip from './Tooltip';

function renderWithSibling(content = 'Full label', disabled = false) {
  return render(
    <>
      <button type="button">Before</button>
      <Tooltip content={content} disabled={disabled}>
        <button type="button">Trigger</button>
      </Tooltip>
      <button type="button">After</button>
    </>,
  );
}

describe('Tooltip', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows its content after a short hover delay and hides when the pointer leaves', () => {
    vi.useFakeTimers();
    renderWithSibling();
    const trigger = screen.getByRole('button', { name: 'Trigger' });

    fireEvent.pointerEnter(trigger);
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(screen.getByRole('tooltip')).toHaveTextContent('Full label');

    fireEvent.pointerLeave(trigger);
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('does not show when the pointer leaves before the delay elapses', () => {
    vi.useFakeTimers();
    renderWithSibling();
    const trigger = screen.getByRole('button', { name: 'Trigger' });

    fireEvent.pointerEnter(trigger);
    fireEvent.pointerLeave(trigger);
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('shows immediately on keyboard focus and wires aria-describedby to the tooltip', async () => {
    const user = userEvent.setup();
    renderWithSibling();
    await user.tab();
    await user.tab();

    const tooltip = screen.getByRole('tooltip');
    expect(tooltip).toHaveTextContent('Full label');
    expect(screen.getByRole('button', { name: 'Trigger' })).toHaveAttribute('aria-describedby', tooltip.id);
  });

  it('hides on blur and on Escape', async () => {
    const user = userEvent.setup();
    renderWithSibling();
    await user.tab();
    await user.tab();
    expect(screen.getByRole('tooltip')).toBeInTheDocument();

    await user.keyboard('{Escape}');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Trigger' })).not.toHaveAttribute('aria-describedby');

    await user.tab({ shift: true });
    await user.tab();
    expect(screen.getByRole('tooltip')).toBeInTheDocument();
    await user.tab();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('renders nothing while disabled', async () => {
    const user = userEvent.setup();
    renderWithSibling('Full label', true);
    await user.tab();
    await user.tab();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it("keeps the trigger's own event handlers", async () => {
    const user = userEvent.setup();
    const onFocus = vi.fn();
    const onBlur = vi.fn();
    render(
      <Tooltip content="Details">
        <button type="button" onFocus={onFocus} onBlur={onBlur}>
          Trigger
        </button>
      </Tooltip>,
    );
    await user.tab();
    await user.tab();
    expect(onFocus).toHaveBeenCalledTimes(1);
    expect(onBlur).toHaveBeenCalledTimes(1);
  });
});
