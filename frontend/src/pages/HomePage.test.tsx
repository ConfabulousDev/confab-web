import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import HomePage from './HomePage';

// useAuth — mutable per test
const mockUseAuth = vi.fn();
vi.mock('@/hooks', () => ({
  useAuth: () => mockUseAuth(),
}));
vi.mock('@/hooks/useDocumentTitle', () => ({
  useDocumentTitle: () => undefined,
}));

// useNavigate — capture the call args
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// HomePage renders these unconditionally; nothing under test depends on
// their contents.
vi.mock('@/components/CTALinks', () => ({ default: () => null }));
vi.mock('@/components/HeroCards', () => ({ default: () => null }));

declare global {
  interface Window {
    __DEMO_IDENTITY__?: unknown;
  }
}

function renderHome() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <HomePage />
    </MemoryRouter>
  );
}

beforeEach(() => {
  mockNavigate.mockClear();
  mockUseAuth.mockReturnValue({ user: null, loading: false, serverError: false });
});

afterEach(() => {
  delete window.__DEMO_IDENTITY__;
});

describe('HomePage post-login redirect', () => {
  it('redirects an authenticated normal user to /sessions?owner=<email>', async () => {
    mockUseAuth.mockReturnValue({
      user: { email: 'alice@example.com' },
      loading: false,
      serverError: false,
    });
    renderHome();
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        `/sessions?owner=${encodeURIComponent('alice@example.com')}`,
        { replace: true }
      );
    });
  });

  it('redirects the demo identity to /sessions (no ?owner=)', async () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    mockUseAuth.mockReturnValue({
      user: { email: 'demo@confabulous.dev' },
      loading: false,
      serverError: false,
    });
    renderHome();
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/sessions', { replace: true });
    });
  });

  it('still pre-fills ?owner= for a non-demo user in a demo deployment', async () => {
    window.__DEMO_IDENTITY__ = 'demo@confabulous.dev';
    mockUseAuth.mockReturnValue({
      user: { email: 'alice@example.com' },
      loading: false,
      serverError: false,
    });
    renderHome();
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        `/sessions?owner=${encodeURIComponent('alice@example.com')}`,
        { replace: true }
      );
    });
  });

  it('does not redirect when loading', () => {
    mockUseAuth.mockReturnValue({ user: null, loading: true, serverError: false });
    renderHome();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('does not redirect when the server is down', () => {
    mockUseAuth.mockReturnValue({
      user: { email: 'alice@example.com' },
      loading: false,
      serverError: true,
    });
    renderHome();
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
