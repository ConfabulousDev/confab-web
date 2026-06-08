import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import Footer from './Footer';

const DOCS_URL = 'https://docs.confabulous.dev';
const GITHUB_ISSUES_URL = 'https://github.com/ConfabulousDev/confab-web/issues';

vi.mock('@/hooks/useAppConfig', () => ({
  useAppConfig: () => ({ supportEmail: 'support@example.com', saasTermlyEnabled: false }),
}));

// CF-571: the SaaS footer surfaces docs and a "Report an issue" link in
// addition to its existing GitHub/Discord/Help/Policies links.
describe('Footer docs & issue links (CF-571)', () => {
  it('renders a Docs link to the docs site', () => {
    render(<Footer />);
    const link = screen.getByRole('link', { name: 'Docs' });
    expect(link).toHaveAttribute('href', DOCS_URL);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders a Report an issue link to the GitHub issues page', () => {
    render(<Footer />);
    const link = screen.getByRole('link', { name: 'Report an issue' });
    expect(link).toHaveAttribute('href', GITHUB_ISSUES_URL);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('keeps the existing GitHub link', () => {
    render(<Footer />);
    expect(screen.getByRole('link', { name: 'GitHub' })).toBeInTheDocument();
  });
});
