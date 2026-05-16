import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import CodexReasoningHidden from './CodexReasoningHidden';
import type { CodexReasoningHiddenItem } from '@/types/codexRenderItem';

vi.mock('./CodexRowActions', () => ({
  default: () => <span data-testid="row-actions" />,
}));

const item: CodexReasoningHiddenItem = {
  kind: 'reasoning_hidden',
  lineId: '3',
  timestamp: '2025-01-01T08:00:00Z',
};

describe('CodexReasoningHidden', () => {
  it('renders "reasoning hidden" label', () => {
    const { getByText } = render(<CodexReasoningHidden item={item} />);
    expect(getByText('reasoning hidden')).toBeInTheDocument();
  });

  it('marks the row with data-kind="reasoning_hidden"', () => {
    const { container } = render(<CodexReasoningHidden item={item} />);
    expect(container.querySelector('[data-kind="reasoning_hidden"]')).not.toBeNull();
  });

  it('renders CodexRowActions only when sessionId is provided', () => {
    const { queryByTestId, rerender } = render(<CodexReasoningHidden item={item} />);
    expect(queryByTestId('row-actions')).toBeNull();

    rerender(<CodexReasoningHidden item={item} sessionId="s1" />);
    expect(queryByTestId('row-actions')).not.toBeNull();
  });

  it.each([
    ['isSelected'],
    ['isDeepLinkTarget'],
    ['isCurrentSearchMatch'],
  ] as const)('applies a class when %s is true', (flag) => {
    const { container } = render(<CodexReasoningHidden item={item} {...{ [flag]: true }} />);
    const row = container.querySelector('[data-kind="reasoning_hidden"]')!;
    // Each flag adds at least one additional CSS-module class beyond the base.
    expect(row.className.split(' ').length).toBeGreaterThanOrEqual(2);
  });
});
