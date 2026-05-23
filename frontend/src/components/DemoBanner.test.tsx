import { afterEach, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import DemoBanner, { REPO_URL, SELF_HOST_URL } from './DemoBanner';

// CF-483: DemoBanner renders only when window.__DEMO_IDENTITY__ is a
// non-empty string. Links point to the documented URLs. Not dismissible.

declare global {
  interface Window {
    __DEMO_IDENTITY__?: unknown;
  }
}

afterEach(() => {
  delete window.__DEMO_IDENTITY__;
});

describe('DemoBanner', () => {
  it('renders nothing when window.__DEMO_IDENTITY__ is unset', () => {
    const { container } = render(<DemoBanner />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when window.__DEMO_IDENTITY__ is empty string', () => {
    window.__DEMO_IDENTITY__ = '';
    const { container } = render(<DemoBanner />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the read-only banner when window.__DEMO_IDENTITY__ is set', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    render(<DemoBanner />);

    // Left text mentions read-only and the identity email.
    expect(screen.getByText(/read-only demo/i)).toBeInTheDocument();
    expect(screen.getByText(/demo@confabulous\.dev/)).toBeInTheDocument();

    // Right links point to documented destinations.
    const selfHost = screen.getByRole('link', { name: /self-host/i });
    expect(selfHost).toHaveAttribute('href', SELF_HOST_URL);
    const repo = screen.getByRole('link', { name: /github/i });
    expect(repo).toHaveAttribute('href', REPO_URL);
  });

  it('has no dismiss control (banner is not dismissible per spec)', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    render(<DemoBanner />);
    expect(screen.queryByRole('button', { name: /close|dismiss|hide/i })).toBeNull();
  });
});
