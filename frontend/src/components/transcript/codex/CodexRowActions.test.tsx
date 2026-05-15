// Spec tests for CodexRowActions (CF-360).
//
// Locks the contract:
//   - copy-link is ALWAYS rendered, builds the exact URL Claude does
//   - copy-text is hidden when copyText is undefined / empty / whitespace
//   - skip buttons are hidden when their callback is undefined
//   - clicks call the right handler

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import CodexRowActions from './CodexRowActions';

// Mock the clipboard API for jsdom — navigator.clipboard isn't defined by default.
const writeText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  writeText.mockClear();
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    writable: true,
    configurable: true,
  });
});

describe('CodexRowActions', () => {
  it('always renders the copy-link button', () => {
    render(<CodexRowActions sessionId="s1" lineId="42" />);
    expect(screen.getByLabelText(/copy link/i)).toBeInTheDocument();
  });

  it('copy-link writes ${origin}/sessions/${sessionId}?tab=transcript&msg=${lineId} to clipboard', () => {
    render(<CodexRowActions sessionId="abc-123" lineId="42" />);
    fireEvent.click(screen.getByLabelText(/copy link/i));
    expect(writeText).toHaveBeenCalledTimes(1);
    expect(writeText).toHaveBeenCalledWith(
      `${window.location.origin}/sessions/abc-123?tab=transcript&msg=42`,
    );
  });

  it('renders copy-text when copyText is non-empty', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" copyText="hello world" />);
    expect(screen.getByLabelText(/copy text/i)).toBeInTheDocument();
  });

  it('hides copy-text when copyText is undefined', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" />);
    expect(screen.queryByLabelText(/copy text/i)).toBeNull();
  });

  it('hides copy-text when copyText is the empty string', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" copyText="" />);
    expect(screen.queryByLabelText(/copy text/i)).toBeNull();
  });

  it('hides copy-text when copyText is whitespace-only', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" copyText={'   \n\t  '} />);
    expect(screen.queryByLabelText(/copy text/i)).toBeNull();
  });

  it('copy-text writes the provided text to clipboard', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" copyText="payload" />);
    fireEvent.click(screen.getByLabelText(/copy text/i));
    expect(writeText).toHaveBeenCalledWith('payload');
  });

  it('renders next-skip when onSkipToNext is provided', () => {
    render(
      <CodexRowActions
        sessionId="s1"
        lineId="0"
        onSkipToNext={() => undefined}
        kindLabel="user prompt"
      />,
    );
    expect(screen.getByLabelText(/next user prompt/i)).toBeInTheDocument();
  });

  it('hides next-skip when onSkipToNext is undefined', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" kindLabel="user prompt" />);
    expect(screen.queryByLabelText(/next user prompt/i)).toBeNull();
  });

  it('renders prev-skip when onSkipToPrevious is provided', () => {
    render(
      <CodexRowActions
        sessionId="s1"
        lineId="0"
        onSkipToPrevious={() => undefined}
        kindLabel="user prompt"
      />,
    );
    expect(screen.getByLabelText(/previous user prompt/i)).toBeInTheDocument();
  });

  it('hides prev-skip when onSkipToPrevious is undefined', () => {
    render(<CodexRowActions sessionId="s1" lineId="0" kindLabel="user prompt" />);
    expect(screen.queryByLabelText(/previous user prompt/i)).toBeNull();
  });

  it('skip-next click fires the provided callback', () => {
    const onSkipToNext = vi.fn();
    render(
      <CodexRowActions
        sessionId="s1"
        lineId="0"
        onSkipToNext={onSkipToNext}
        kindLabel="exec command"
      />,
    );
    fireEvent.click(screen.getByLabelText(/next exec command/i));
    expect(onSkipToNext).toHaveBeenCalledTimes(1);
  });

  it('skip-prev click fires the provided callback', () => {
    const onSkipToPrevious = vi.fn();
    render(
      <CodexRowActions
        sessionId="s1"
        lineId="0"
        onSkipToPrevious={onSkipToPrevious}
        kindLabel="exec command"
      />,
    );
    fireEvent.click(screen.getByLabelText(/previous exec command/i));
    expect(onSkipToPrevious).toHaveBeenCalledTimes(1);
  });

  it('uses "row" as the default kindLabel when none provided', () => {
    render(
      <CodexRowActions
        sessionId="s1"
        lineId="0"
        onSkipToNext={() => undefined}
      />,
    );
    expect(screen.getByLabelText(/next row/i)).toBeInTheDocument();
  });
});
