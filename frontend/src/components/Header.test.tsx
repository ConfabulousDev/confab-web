import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Header from './Header';

vi.mock('@/hooks/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('@/hooks/useAppConfig', () => ({
  useAppConfig: () => ({ sharesEnabled: false, orgAnalyticsEnabled: false, version: null }),
}));
// ThemeToggle / UpdateBadge each require their own context; mock to keep
// the test focused on what Header itself renders.
vi.mock('./ThemeToggle', () => ({ default: () => null }));
vi.mock('./UpdateBadge', () => ({ default: () => null }));

import { useAuth } from '@/hooks/useAuth';

declare global {
  interface Window {
    __DEMO_IDENTITY__?: unknown;
  }
}

beforeEach(() => {
  vi.mocked(useAuth).mockReturnValue({
    user: null,
    loading: false,
    error: null,
    isAuthenticated: false,
    serverError: false,
    refetch: vi.fn(),
  });
});

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

function signInAs(email: string) {
  vi.mocked(useAuth).mockReturnValue({
    user: { email },
    loading: false,
    error: null,
    isAuthenticated: true,
    serverError: false,
    refetch: vi.fn(),
  });
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

describe('Header nav links — owner pre-filter', () => {
  it('Sessions link pre-fills ?owner=<email> for a normal authenticated user', () => {
    signInAs('alice@example.com');
    renderHeader();
    const link = screen.getByRole('link', { name: 'Sessions' });
    expect(link.getAttribute('href')).toBe(
      `/sessions?owner=${encodeURIComponent('alice@example.com')}`
    );
  });

  it('TILs link pre-fills ?owner=<email> for a normal authenticated user', () => {
    signInAs('alice@example.com');
    renderHeader();
    const link = screen.getByRole('link', { name: 'TILs' });
    expect(link.getAttribute('href')).toBe(
      `/tils?owner=${encodeURIComponent('alice@example.com')}`
    );
  });

  it('Sessions link omits ?owner= when the current user IS the demo identity', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    signInAs('demo@confabulous.dev');
    renderHeader();
    const link = screen.getByRole('link', { name: 'Sessions' });
    expect(link.getAttribute('href')).toBe('/sessions');
  });

  it('TILs link omits ?owner= when the current user IS the demo identity', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    signInAs('demo@confabulous.dev');
    renderHeader();
    const link = screen.getByRole('link', { name: 'TILs' });
    expect(link.getAttribute('href')).toBe('/tils');
  });

  it('Sessions link still pre-fills ?owner= when demo mode is on but the user is NOT the demo identity', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    signInAs('alice@example.com');
    renderHeader();
    const link = screen.getByRole('link', { name: 'Sessions' });
    expect(link.getAttribute('href')).toBe(
      `/sessions?owner=${encodeURIComponent('alice@example.com')}`
    );
  });

  it('TILs link still pre-fills ?owner= when demo mode is on but the user is NOT the demo identity', () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    signInAs('alice@example.com');
    renderHeader();
    const link = screen.getByRole('link', { name: 'TILs' });
    expect(link.getAttribute('href')).toBe(
      `/tils?owner=${encodeURIComponent('alice@example.com')}`
    );
  });
});
