import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import TILBadge from './TILBadge';
import type { TIL } from '@/schemas/api';

function til(id: number, title: string, summary: string): TIL {
  return {
    id,
    title,
    summary,
    session_id: 'sess-1',
    created_at: '2025-01-01T00:00:00Z',
  };
}

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('TILBadge', () => {
  it('returns null when tils is empty', () => {
    const { container } = renderWithRouter(<TILBadge tils={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders label "TIL" for a single item', () => {
    const { getByRole } = renderWithRouter(<TILBadge tils={[til(1, 't', 's')]} />);
    expect(getByRole('button')).toHaveTextContent('TIL');
    expect(getByRole('button')).not.toHaveTextContent('(');
  });

  it('renders label "TIL (3)" when multiple items', () => {
    const tils = [til(1, 'a', 'a'), til(2, 'b', 'b'), til(3, 'c', 'c')];
    const { getByRole } = renderWithRouter(<TILBadge tils={tils} />);
    expect(getByRole('button')).toHaveTextContent('TIL (3)');
  });

  it('opens popover on click and lists titles + summaries', async () => {
    const user = userEvent.setup();
    const tils = [til(1, 'First TIL', 'Summary one'), til(2, 'Second TIL', 'Summary two')];
    const { getByRole, getByText } = renderWithRouter(<TILBadge tils={tils} />);
    await user.click(getByRole('button'));

    expect(getByText('First TIL')).toBeInTheDocument();
    expect(getByText('Summary one')).toBeInTheDocument();
    expect(getByText('Second TIL')).toBeInTheDocument();
    expect(getByText('Summary two')).toBeInTheDocument();
  });

  it('renders "View in TILs" link pointing to /tils when open', async () => {
    const user = userEvent.setup();
    const { getByRole, getByText } = renderWithRouter(
      <TILBadge tils={[til(1, 'a', 'a')]} />
    );
    await user.click(getByRole('button'));
    const link = getByText('View in TILs').closest('a');
    expect(link).toHaveAttribute('href', '/tils');
  });

  it('stops click propagation on popover clicks', async () => {
    const user = userEvent.setup();
    const parentClick = vi.fn();
    const { getByRole, getByText } = renderWithRouter(
      <div onClick={parentClick}>
        <TILBadge tils={[til(1, 'a', 'summary text')]} />
      </div>
    );

    await user.click(getByRole('button'));
    parentClick.mockClear();

    await user.click(getByText('summary text'));
    expect(parentClick).not.toHaveBeenCalled();
  });
});
