// 6h7m: shared divider row used by all 4 transcript providers.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TimeSeparator } from './TimeSeparator';

describe('TimeSeparator', () => {
  it('renders the label text', () => {
    render(<TimeSeparator label="Tuesday, July 7" />);
    expect(screen.getByText('Tuesday, July 7')).toBeInTheDocument();
  });

  it('does not render the estimated tilde or tooltip by default', () => {
    const { container } = render(<TimeSeparator label="6:00 PM" />);
    expect(container.textContent).not.toContain('~');
    expect(container.querySelector('[title]')).toBeNull();
  });

  it('renders the estimated tilde prefix and tooltip when estimated is true', () => {
    const { container } = render(<TimeSeparator label="6:00 PM" estimated />);
    expect(container.textContent).toContain('~');
    expect(container.textContent).toContain('6:00 PM');
    const withTitle = container.querySelector('[title]');
    expect(withTitle).not.toBeNull();
    expect(withTitle?.getAttribute('title')).toMatch(/estimated/i);
  });
});
