import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Alert from './Alert';

describe('Alert', () => {
  it('renders children correctly', () => {
    render(<Alert>This is an alert message</Alert>);
    expect(screen.getByText('This is an alert message')).toBeInTheDocument();
  });

  it('applies variant classes', () => {
    const { container, rerender } = render(<Alert variant="info">Info</Alert>);
    // The alert is the first child of the container
    const alert = container.firstElementChild;
    expect(alert?.className).toMatch(/info/);

    rerender(<Alert variant="success">Success</Alert>);
    expect(container.firstElementChild?.className).toMatch(/success/);

    rerender(<Alert variant="error">Error</Alert>);
    expect(container.firstElementChild?.className).toMatch(/error/);

    rerender(<Alert variant="warning">Warning</Alert>);
    expect(container.firstElementChild?.className).toMatch(/warning/);
  });

  it('defaults to info variant', () => {
    const { container } = render(<Alert>Default</Alert>);
    expect(container.firstElementChild?.className).toMatch(/info/);
  });

  it('shows close button when onClose is provided', () => {
    render(<Alert onClose={() => {}}>Closeable</Alert>);
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument();
  });

  it('does not show close button when onClose is not provided', () => {
    render(<Alert>Not closeable</Alert>);
    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument();
  });

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup();
    const handleClose = vi.fn();
    render(<Alert onClose={handleClose}>Closeable</Alert>);

    await user.click(screen.getByRole('button', { name: 'Close' }));
    expect(handleClose).toHaveBeenCalledTimes(1);
  });

  it('merges custom className', () => {
    const { container } = render(<Alert className="custom-alert">Custom</Alert>);
    expect(container.firstElementChild).toHaveClass('custom-alert');
  });
});
