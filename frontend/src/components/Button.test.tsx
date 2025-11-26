import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Button from './Button';

describe('Button', () => {
  it('renders children correctly', () => {
    render(<Button>Click me</Button>);
    expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument();
  });

  it('handles click events', async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(<Button onClick={handleClick}>Click me</Button>);

    await user.click(screen.getByRole('button'));
    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it('can be disabled', async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(
      <Button onClick={handleClick} disabled>
        Disabled
      </Button>
    );

    const button = screen.getByRole('button');
    expect(button).toBeDisabled();

    await user.click(button);
    expect(handleClick).not.toHaveBeenCalled();
  });

  it('applies variant classes', () => {
    const { rerender } = render(<Button variant="primary">Primary</Button>);
    expect(screen.getByRole('button').className).toMatch(/primary/);

    rerender(<Button variant="secondary">Secondary</Button>);
    expect(screen.getByRole('button').className).toMatch(/secondary/);

    rerender(<Button variant="danger">Danger</Button>);
    expect(screen.getByRole('button').className).toMatch(/danger/);

    rerender(<Button variant="github">GitHub</Button>);
    expect(screen.getByRole('button').className).toMatch(/github/);
  });

  it('applies size classes', () => {
    const { rerender } = render(<Button size="default">Default</Button>);
    expect(screen.getByRole('button').className).not.toMatch(/\bsm\b/);

    rerender(<Button size="sm">Small</Button>);
    expect(screen.getByRole('button').className).toMatch(/sm/);
  });

  it('merges custom className', () => {
    render(<Button className="custom-class">Custom</Button>);
    expect(screen.getByRole('button')).toHaveClass('custom-class');
  });

  it('passes through HTML button attributes', () => {
    render(
      <Button type="submit" name="submit-btn">
        Submit
      </Button>
    );
    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('type', 'submit');
    expect(button).toHaveAttribute('name', 'submit-btn');
  });
});
