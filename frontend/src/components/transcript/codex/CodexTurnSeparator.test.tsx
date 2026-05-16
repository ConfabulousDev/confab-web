import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import CodexTurnSeparator from './CodexTurnSeparator';
import type { CodexTurnSeparatorItem } from '@/types/codexRenderItem';

vi.mock('./CodexRowActions', () => ({
  default: () => <span data-testid="row-actions" />,
}));

function makeItem(overrides: Partial<CodexTurnSeparatorItem> = {}): CodexTurnSeparatorItem {
  return {
    kind: 'turn_separator',
    lineId: '5',
    timestamp: '2025-01-01T12:00:00Z',
    turnIndex: 3,
    durationMs: 45_000,
    ...overrides,
  };
}

describe('CodexTurnSeparator', () => {
  it('renders Turn N label using item.turnIndex', () => {
    const { getByText } = render(<CodexTurnSeparator item={makeItem({ turnIndex: 7 })} />);
    expect(getByText('Turn 7')).toBeInTheDocument();
  });

  it('renders formatted duration', () => {
    const { container } = render(
      <CodexTurnSeparator item={makeItem({ durationMs: 45_000 })} />
    );
    expect(container.textContent).toContain('45s');
  });

  it('includes TTFT segment when timeToFirstTokenMs is defined', () => {
    const { container } = render(
      <CodexTurnSeparator
        item={makeItem({ durationMs: 45_000, timeToFirstTokenMs: 250 })}
      />
    );
    expect(container.textContent).toContain('TTFT 250ms');
  });

  it('omits TTFT segment when timeToFirstTokenMs is undefined', () => {
    const { container } = render(<CodexTurnSeparator item={makeItem()} />);
    expect(container.textContent).not.toContain('TTFT');
  });

  it('renders CodexRowActions only when sessionId is provided', () => {
    const { queryByTestId, rerender } = render(
      <CodexTurnSeparator item={makeItem()} />
    );
    expect(queryByTestId('row-actions')).toBeNull();
    rerender(<CodexTurnSeparator item={makeItem()} sessionId="s1" />);
    expect(queryByTestId('row-actions')).not.toBeNull();
  });

  it('marks the row with data-kind="turn_separator"', () => {
    const { container } = render(<CodexTurnSeparator item={makeItem()} />);
    expect(container.querySelector('[data-kind="turn_separator"]')).not.toBeNull();
  });
});
