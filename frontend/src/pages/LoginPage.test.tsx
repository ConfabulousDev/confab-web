import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import LoginPage from './LoginPage';

// Mock useAuth
const mockUseAuth = vi.fn();
vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Standard 2-provider config (avoids single-provider auto-redirect)
const twoProviders = [
  { name: 'github', display_name: 'GitHub', login_url: '/auth/github/login' },
  { name: 'google', display_name: 'Google', login_url: '/auth/google/login' },
];

function renderWithRouter(initialEntry = '/login') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LoginPage />
    </MemoryRouter>
  );
}

describe('LoginPage', () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    mockUseAuth.mockReturnValue({
      isAuthenticated: false,
      loading: false,
      user: null,
    });
    mockNavigate.mockClear();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  function mockFetchConfig(providers: Array<{ name: string; display_name: string; login_url: string }>) {
    globalThis.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve({ providers }),
    });
  }

  it('renders OAuth provider buttons', async () => {
    mockFetchConfig(twoProviders);

    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('Continue with GitHub')).toBeInTheDocument();
    });
    expect(screen.getByText('Continue with Google')).toBeInTheDocument();
  });

  it('renders password form when password provider is present', async () => {
    mockFetchConfig([
      { name: 'password', display_name: 'Password', login_url: '/auth/password/login' },
      { name: 'github', display_name: 'GitHub', login_url: '/auth/github/login' },
    ]);

    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Email')).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument();
    expect(screen.getByText('Sign in')).toBeInTheDocument();
    expect(screen.getByText('or continue with')).toBeInTheDocument();
  });

  it('does not show divider with only OAuth providers', async () => {
    mockFetchConfig(twoProviders);

    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('Continue with GitHub')).toBeInTheDocument();
    });
    expect(screen.queryByText('or continue with')).not.toBeInTheDocument();
  });

  it('displays error from URL params', async () => {
    mockFetchConfig(twoProviders);

    renderWithRouter('/login?error=github_error&error_description=Something+went+wrong');

    await waitFor(() => {
      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    });
  });

  it('displays access denied with contact link', async () => {
    mockFetchConfig(twoProviders);

    renderWithRouter('/login?error=access_denied&error_description=User+limit+reached');

    await waitFor(() => {
      expect(screen.getByText(/Please request access/)).toBeInTheDocument();
    });
  });

  it('redirects to /sessions when already authenticated', async () => {
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      loading: false,
      user: { id: 1, email: 'test@example.com' },
    });

    mockFetchConfig(twoProviders);

    renderWithRouter();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/sessions', { replace: true });
    });
  });

  it('renders nothing while auth is loading', () => {
    mockUseAuth.mockReturnValue({
      isAuthenticated: false,
      loading: true,
      user: null,
    });

    mockFetchConfig([]);

    const { container } = renderWithRouter();
    expect(container.innerHTML).toBe('');
  });

  it('shows email hint in subtitle when email param present', async () => {
    mockFetchConfig(twoProviders);

    renderWithRouter('/login?email=alice@example.com');

    await waitFor(() => {
      expect(screen.getByText(/Sign in with/)).toBeInTheDocument();
    });
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });

  it('passes redirect and email to OAuth URLs', async () => {
    mockFetchConfig(twoProviders);

    renderWithRouter('/login?redirect=%2Fsessions%2F123&email=alice@example.com');

    await waitFor(() => {
      const link = screen.getByText('Continue with GitHub').closest('a');
      expect(link).toHaveAttribute(
        'href',
        expect.stringContaining('/auth/github/login?')
      );
      expect(link?.getAttribute('href')).toContain('redirect=%2Fsessions%2F123');
      expect(link?.getAttribute('href')).toContain('email=alice%40example.com');
    });
  });

  it('renders OIDC provider with custom display name', async () => {
    mockFetchConfig([
      { name: 'oidc', display_name: 'Okta', login_url: '/auth/oidc/login' },
      { name: 'github', display_name: 'GitHub', login_url: '/auth/github/login' },
    ]);

    renderWithRouter();

    await waitFor(() => {
      expect(screen.getByText('Continue with Okta')).toBeInTheDocument();
    });
  });

  it('returns null for single OAuth provider (auto-redirect)', async () => {
    mockFetchConfig([
      { name: 'github', display_name: 'GitHub', login_url: '/auth/github/login' },
    ]);

    const { container } = renderWithRouter();

    // Wait for config to load, then component returns null for auto-redirect
    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalled();
    });
    // Component should render nothing (auto-redirect case)
    expect(container.innerHTML).toBe('');
  });
});
