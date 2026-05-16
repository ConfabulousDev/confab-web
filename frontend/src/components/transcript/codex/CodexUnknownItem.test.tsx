import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import CodexUnknownItem from './CodexUnknownItem';
import type { CodexUnknownItem as CodexUnknownItemType } from '@/types/codexRenderItem';

vi.mock('./CodexRowActions', () => ({
  default: () => <span data-testid="row-actions" />,
}));

const baseItem: CodexUnknownItemType = {
  kind: 'unknown',
  lineId: '7',
  timestamp: '2025-01-01T12:34:56Z',
  rawLine: { foo: 'bar', nested: { value: 42 } },
};

describe('CodexUnknownItem', () => {
  it('renders "Unrecognized line" label', () => {
    const { getByText } = render(<CodexUnknownItem item={baseItem} />);
    expect(getByText('Unrecognized line')).toBeInTheDocument();
  });

  it('renders raw line JSON inside <pre>', () => {
    const { container } = render(<CodexUnknownItem item={baseItem} />);
    const pre = container.querySelector('pre');
    expect(pre).not.toBeNull();
    expect(pre?.textContent).toContain('"foo"');
    expect(pre?.textContent).toContain('"bar"');
  });

  it('toggling <details> open updates the controlled state', () => {
    const { container } = render(<CodexUnknownItem item={baseItem} />);
    const details = container.querySelector('details')!;
    expect(details.open).toBe(false);

    details.open = true;
    fireEvent(details, new Event('toggle'));
    expect(details.open).toBe(true);

    details.open = false;
    fireEvent(details, new Event('toggle'));
    expect(details.open).toBe(false);
  });

  it('auto-opens when searchQuery matches raw content', () => {
    const { container, rerender } = render(<CodexUnknownItem item={baseItem} searchQuery="" />);
    const details = container.querySelector('details')!;
    expect(details.open).toBe(false);

    rerender(<CodexUnknownItem item={baseItem} searchQuery="bar" />);
    expect(details.open).toBe(true);
  });

  it('stays open after searchQuery clears (user-controlled close only)', () => {
    const { container, rerender } = render(<CodexUnknownItem item={baseItem} searchQuery="" />);
    rerender(<CodexUnknownItem item={baseItem} searchQuery="bar" />);
    const details = container.querySelector('details')!;
    expect(details.open).toBe(true);

    rerender(<CodexUnknownItem item={baseItem} searchQuery="" />);
    expect(details.open).toBe(true);
  });

  it('wraps matching text in <mark> with highlight class', () => {
    const { container } = render(<CodexUnknownItem item={baseItem} searchQuery="foo" />);
    const mark = container.querySelector('mark');
    expect(mark).not.toBeNull();
    expect(mark?.textContent).toBe('foo');
  });

  it('renders CodexRowActions when sessionId provided', () => {
    const { getByTestId } = render(
      <CodexUnknownItem item={baseItem} sessionId="sess-1" />
    );
    expect(getByTestId('row-actions')).toBeInTheDocument();
  });

  it('omits CodexRowActions when sessionId is absent', () => {
    const { queryByTestId } = render(<CodexUnknownItem item={baseItem} />);
    expect(queryByTestId('row-actions')).toBeNull();
  });
});
