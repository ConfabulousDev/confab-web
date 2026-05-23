import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ReadOnlyToast, { TOAST_DURATION_MS, TOAST_TEXT } from './ReadOnlyToast';
import { READ_ONLY_EVENT } from '@/utils/demoIdentity';

// CF-483: ReadOnlyToast shows on the documented CustomEvent, holds the
// toast for ~TOAST_DURATION_MS, then dismisses. Re-firing the event
// while visible must reset the timer (debounced replace) — a stack of
// toasts is explicitly out of scope per the interview.

describe('ReadOnlyToast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  function fireEvent() {
    act(() => {
      window.dispatchEvent(new CustomEvent(READ_ONLY_EVENT));
    });
  }

  it('does not render anything before any event fires', () => {
    render(<ReadOnlyToast />);
    expect(screen.queryByText(TOAST_TEXT)).toBeNull();
  });

  it('shows the toast text on the read-only event', () => {
    render(<ReadOnlyToast />);
    fireEvent();
    expect(screen.getByText(TOAST_TEXT)).toBeInTheDocument();
  });

  it('auto-dismisses after TOAST_DURATION_MS', () => {
    render(<ReadOnlyToast />);
    fireEvent();
    expect(screen.getByText(TOAST_TEXT)).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(TOAST_DURATION_MS + 50);
    });
    expect(screen.queryByText(TOAST_TEXT)).toBeNull();
  });

  it('debounces (resets timer) when re-fired before dismissal', () => {
    render(<ReadOnlyToast />);
    fireEvent();
    act(() => {
      vi.advanceTimersByTime(TOAST_DURATION_MS - 500);
    });
    // Still visible — and re-firing should keep it visible past the
    // original deadline.
    fireEvent();
    act(() => {
      vi.advanceTimersByTime(TOAST_DURATION_MS - 500);
    });
    expect(screen.getByText(TOAST_TEXT)).toBeInTheDocument();
    // Finally let it expire.
    act(() => {
      vi.advanceTimersByTime(TOAST_DURATION_MS);
    });
    expect(screen.queryByText(TOAST_TEXT)).toBeNull();
  });
});
