import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TokensCard } from './TokensCard';

const mockData = {
  input: 110000,
  output: 20800,
  cache_creation: 23000,
  cache_read: 36000,
  estimated_usd: '4.25',
};

describe('TokensCard', () => {
  it('renders all token stats with formatted values', () => {
    render(<TokensCard data={mockData} loading={false} />);

    expect(screen.getByText('Tokens')).toBeInTheDocument();
    expect(screen.getByText('Input')).toBeInTheDocument();
    expect(screen.getByText('110.0k')).toBeInTheDocument();
    expect(screen.getByText('Output')).toBeInTheDocument();
    expect(screen.getByText('20.8k')).toBeInTheDocument();
    expect(screen.getByText('Cache created')).toBeInTheDocument();
    expect(screen.getByText('23.0k')).toBeInTheDocument();
    expect(screen.getByText('Cache read')).toBeInTheDocument();
    expect(screen.getByText('36.0k')).toBeInTheDocument();
    expect(screen.getByText('Estimated cost')).toBeInTheDocument();
    expect(screen.getByText('$4.25')).toBeInTheDocument();
  });

  it('shows loading state when loading with no data', () => {
    render(<TokensCard data={null} loading={true} />);

    expect(screen.getByText('Tokens')).toBeInTheDocument();
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('returns null when not loading and no data', () => {
    const { container } = render(<TokensCard data={null} loading={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('shows data even while loading (optimistic update)', () => {
    render(<TokensCard data={mockData} loading={true} />);

    expect(screen.getByText('110.0k')).toBeInTheDocument();
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  });

  it('shows warning tooltip when cost is zero', () => {
    const zeroCostData = { ...mockData, estimated_usd: '0.00' };
    render(<TokensCard data={zeroCostData} loading={false} />);

    expect(screen.getByText('$0.00')).toBeInTheDocument();
    const costRow = screen.getByText('Estimated cost').closest('div');
    expect(costRow).toHaveAttribute('title', 'Cost unavailable — session may use models not yet in the pricing table');
  });
});
