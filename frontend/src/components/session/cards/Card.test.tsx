import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CardWrapper, StatRow, CardLoading } from './Card';

describe('CardWrapper', () => {
  it('renders title and children', () => {
    render(
      <CardWrapper title="Test Card">
        <div data-testid="child">Content</div>
      </CardWrapper>
    );

    expect(screen.getByText('Test Card')).toBeInTheDocument();
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });
});

describe('StatRow', () => {
  it('renders label and value', () => {
    render(<StatRow label="Input" value="1,234" />);

    expect(screen.getByText('Input')).toBeInTheDocument();
    expect(screen.getByText('1,234')).toBeInTheDocument();
  });

  it('renders with icon', () => {
    render(<StatRow label="Test" value="123" icon={<span data-testid="icon">→</span>} />);

    expect(screen.getByTestId('icon')).toBeInTheDocument();
  });

  it('applies tooltip as title attribute', () => {
    render(<StatRow label="Test" value="123" tooltip="Helpful tip" />);

    const row = screen.getByText('Test').closest('div');
    expect(row).toHaveAttribute('title', 'Helpful tip');
  });

  it('applies custom value className', () => {
    render(<StatRow label="Cost" value="$1.50" valueClassName="custom-cost" />);

    const value = screen.getByText('$1.50');
    expect(value).toHaveClass('custom-cost');
  });
});

describe('CardLoading', () => {
  it('renders loading text', () => {
    render(<CardLoading />);

    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });
});
