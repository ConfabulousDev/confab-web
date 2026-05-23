import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Header from './Header';

// Mock the hooks Header reads from so we can render the component in
// isolation without a QueryClient or AppConfigProvider.
vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({ user: null, isAuthenticated: false, loading: false }),
}));
vi.mock('@/hooks/useAppConfig', () => ({
  useAppConfig: () => ({ sharesEnabled: false, orgAnalyticsEnabled: false, version: null }),
}));
// ThemeToggle / UpdateBadge each require their own context; mock to keep
// the test focused on the badge in the logo.
vi.mock('./ThemeToggle', () => ({ default: () => null }));
vi.mock('./UpdateBadge', () => ({ default: () => null }));

declare global {
  interface Window {
    __DEMO_IDENTITY__?: unknown;
  }
}

afterEach(() => {
  delete window.__DEMO_IDENTITY__;
});

function renderHeader() {
  return render(
    <MemoryRouter>
      <Header />
    </MemoryRouter>
  );
}

describe('Header logo badge', () => {
  // Beta badge removal: normal deployments must not show any badge next
  // to the logo. Previously the CSS pseudo-element forced 'beta'.
  it('renders no badge in normal (non-demo) deployments', () => {
    renderHeader();
    expect(screen.getByText('Confabulous')).toBeInTheDocument();
    expect(screen.queryByText(/beta/i)).toBeNull();
    expect(screen.queryByText(/demo/i)).toBeNull();
  });

  it('renders a "demo" badge when window.__DEMO_IDENTITY__ is set', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    renderHeader();
    expect(screen.getByText('Confabulous')).toBeInTheDocument();
    expect(screen.getByText('demo')).toBeInTheDocument();
  });

  it('still renders no badge when window.__DEMO_IDENTITY__ is an empty string', () => {
    window.__DEMO_IDENTITY__ = '';
    renderHeader();
    expect(screen.queryByText(/demo/i)).toBeNull();
  });
});
