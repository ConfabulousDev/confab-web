// CF-368: divider emitted when a Codex turn ends via `event_msg.turn_aborted`
// (user pressed Esc, etc.). Mirrors `CodexCompactedDivider`'s shape.

import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import CodexTurnAbortedDivider from './CodexTurnAbortedDivider';
import type { CodexTurnAbortedItem } from '@/types/codexRenderItem';

vi.mock('./CodexRowActions', () => ({
  default: () => <span data-testid="row-actions" />,
}));

function makeItem(overrides: Partial<CodexTurnAbortedItem> = {}): CodexTurnAbortedItem {
  return {
    kind: 'turn_aborted',
    lineId: '4',
    timestamp: '2026-05-13T12:00:00Z',
    reason: 'interrupted',
    durationMs: 4_000,
    ...overrides,
  };
}

describe('CodexTurnAbortedDivider', () => {
  it('marks the row with data-kind="turn_aborted"', () => {
    const { container } = render(<CodexTurnAbortedDivider item={makeItem()} />);
    expect(container.querySelector('[data-kind="turn_aborted"]')).not.toBeNull();
  });

  it('renders "Turn aborted · <reason> · <duration>" when both are present', () => {
    const { container } = render(
      <CodexTurnAbortedDivider item={makeItem({ reason: 'interrupted', durationMs: 4_000 })} />,
    );
    expect(container.textContent).toContain('Turn aborted');
    expect(container.textContent).toContain('interrupted');
    expect(container.textContent).toContain('4s');
  });

  it('omits the reason segment when reason is empty', () => {
    const { container } = render(
      <CodexTurnAbortedDivider item={makeItem({ reason: '', durationMs: 4_000 })} />,
    );
    expect(container.textContent).toContain('Turn aborted');
    expect(container.textContent).toContain('4s');
    // No trailing " ·  · " from a blanked-out reason.
    expect(container.textContent).not.toMatch(/Turn aborted\s*·\s*·/);
  });

  it('omits the duration segment when durationMs is 0', () => {
    const { container } = render(
      <CodexTurnAbortedDivider item={makeItem({ reason: 'interrupted', durationMs: 0 })} />,
    );
    expect(container.textContent).toContain('Turn aborted');
    expect(container.textContent).toContain('interrupted');
    expect(container.textContent).not.toMatch(/0\s*ms/);
    expect(container.textContent).not.toMatch(/0\s*s/);
  });

  it('renders just "Turn aborted" when reason is empty and durationMs is 0', () => {
    const { container } = render(
      <CodexTurnAbortedDivider item={makeItem({ reason: '', durationMs: 0 })} />,
    );
    const root = container.querySelector('[data-kind="turn_aborted"]');
    expect(root).not.toBeNull();
    // Visible text on the divider should be "Turn aborted" plus the timestamp,
    // with no leftover separators.
    expect(root?.textContent).toMatch(/Turn aborted/);
    expect(root?.textContent).not.toMatch(/Turn aborted\s*·/);
  });

  it('renders CodexRowActions only when sessionId is provided', () => {
    const { queryByTestId, rerender } = render(<CodexTurnAbortedDivider item={makeItem()} />);
    expect(queryByTestId('row-actions')).toBeNull();
    rerender(<CodexTurnAbortedDivider item={makeItem()} sessionId="s1" />);
    expect(queryByTestId('row-actions')).not.toBeNull();
  });

  it('applies the selected class when isSelected is true', () => {
    const { container } = render(<CodexTurnAbortedDivider item={makeItem()} isSelected />);
    expect(container.firstChild).toHaveClass(/selected/);
  });

  it('applies the deepLinkTarget class when isDeepLinkTarget is true', () => {
    const { container } = render(
      <CodexTurnAbortedDivider item={makeItem()} isDeepLinkTarget />,
    );
    expect(container.firstChild).toHaveClass(/deepLinkTarget/);
  });
});
