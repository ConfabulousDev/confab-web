import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TokensCard } from './TokensCard';

const mockData = {
  input: 110000,
  output: 20800,
  cache_creation: 23000,
  cache_read: 36000,
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
});
