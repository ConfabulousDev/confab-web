import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CostCard } from './CostCard';

describe('CostCard', () => {
  it('renders cost with formatted value', () => {
    render(<CostCard data={{ estimated_usd: '4.23' }} loading={false} />);

    expect(screen.getByText('Cost')).toBeInTheDocument();
    expect(screen.getByText('Estimated')).toBeInTheDocument();
    expect(screen.getByText('$4.23')).toBeInTheDocument();
  });

  it('formats large costs correctly', () => {
    render(<CostCard data={{ estimated_usd: '127.45' }} loading={false} />);

    expect(screen.getByText('$127.45')).toBeInTheDocument();
  });

  it('formats small costs as less than $0.01', () => {
    render(<CostCard data={{ estimated_usd: '0.00' }} loading={false} />);

    expect(screen.getByText('<$0.01')).toBeInTheDocument();
  });

  it('shows loading state when loading with no data', () => {
    render(<CostCard data={null} loading={true} />);

    expect(screen.getByText('Cost')).toBeInTheDocument();
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('returns null when not loading and no data', () => {
    const { container } = render(<CostCard data={null} loading={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('shows data even while loading (optimistic update)', () => {
    render(<CostCard data={{ estimated_usd: '4.23' }} loading={true} />);

    expect(screen.getByText('$4.23')).toBeInTheDocument();
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  });
});
