import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import CTALinks from './CTALinks';

describe('CTALinks', () => {
  it('renders Demo, Docs, and GitHub links in that order', () => {
    render(<CTALinks />);
    const links = screen.getAllByRole('link');
    expect(links).toHaveLength(3);
    expect(links[0]).toHaveTextContent(/^Demo/);
    expect(links[1]).toHaveTextContent(/^Docs/);
    expect(links[2]).toHaveTextContent(/^GitHub/);
  });

  it('points Demo at demo.confabulous.dev', () => {
    render(<CTALinks />);
    expect(screen.getByRole('link', { name: /^Demo/ })).toHaveAttribute(
      'href',
      'https://demo.confabulous.dev',
    );
  });

  it('points Docs at the Introduction page on docs.confabulous.dev', () => {
    render(<CTALinks />);
    expect(screen.getByRole('link', { name: /^Docs/ })).toHaveAttribute(
      'href',
      'https://docs.confabulous.dev/getting-started/introduction/',
    );
  });

  it('points GitHub at the confab-web repo', () => {
    render(<CTALinks />);
    expect(screen.getByRole('link', { name: /^GitHub/ })).toHaveAttribute(
      'href',
      'https://github.com/ConfabulousDev/confab-web',
    );
  });

  it('opens every link in a new tab with safe rel attributes', () => {
    render(<CTALinks />);
    for (const link of screen.getAllByRole('link')) {
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    }
  });
});
