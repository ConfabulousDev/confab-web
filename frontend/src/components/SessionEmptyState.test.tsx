import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import SessionEmptyState from './SessionEmptyState';

const DOCS_URL = 'https://docs.confabulous.dev';

// CF-571: the empty state nudges newcomers toward the docs site.
describe('SessionEmptyState docs link (CF-571)', () => {
  it('renders a docs link to the docs site', () => {
    render(<SessionEmptyState />);
    const link = screen.getByRole('link', { name: /docs/i });
    expect(link).toHaveAttribute('href', DOCS_URL);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('still shows the existing no-sessions message', () => {
    render(<SessionEmptyState />);
    expect(screen.getByText('No sessions match the selected filters.')).toBeInTheDocument();
  });
});
