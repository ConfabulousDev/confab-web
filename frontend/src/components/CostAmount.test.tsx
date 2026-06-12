import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CostAmount } from './CostAmount';

describe('CostAmount', () => {
  it('renders the formatCost output for a normal amount', () => {
    render(<CostAmount usd={1.23} />);
    expect(screen.getByText('$1.23')).toBeInTheDocument();
  });

  it('applies the green cost class for a non-zero amount', () => {
    const { container } = render(<CostAmount usd={1.23} />);
    const span = container.firstElementChild;
    expect(span?.className).toMatch(/cost/);
    expect(span?.className).not.toMatch(/zero/);
  });

  it('renders tiny non-zero amounts as "<$0.01" and keeps them green (not the zero class)', () => {
    const { container } = render(<CostAmount usd={0.004} />);
    expect(screen.getByText('<$0.01')).toBeInTheDocument();
    const span = container.firstElementChild;
    expect(span?.className).toMatch(/cost/);
    expect(span?.className).not.toMatch(/zero/);
  });

  it('applies the amber zero class for exactly $0.00', () => {
    const { container } = render(<CostAmount usd={0} />);
    expect(screen.getByText('$0.00')).toBeInTheDocument();
    const span = container.firstElementChild;
    expect(span?.className).toMatch(/zero/);
  });

  it('forwards a passed className while keeping the cost class', () => {
    const { container } = render(<CostAmount usd={1.23} className="myLayout" />);
    const span = container.firstElementChild;
    expect(span?.className).toMatch(/cost/);
    expect(span?.className).toMatch(/myLayout/);
  });
});
