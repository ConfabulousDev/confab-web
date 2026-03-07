import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useAuth } from './useAuth';
import type { ReactNode } from 'react';

// Mock the API module
vi.mock('@/services/api', () => ({
  authAPI: {
    me: vi.fn(),
  },
  AuthenticationError: class AuthenticationError extends Error {
    constructor(message = 'Authentication required') {
      super(message);
      this.name = 'AuthenticationError';
    }
  },
}));

import { authAPI, AuthenticationError } from '@/services/api';

// Create a wrapper with QueryClient for each test
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe('useAuth', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns loading state initially', () => {
    vi.mocked(authAPI.me).mockImplementation(() => new Promise(() => {})); // Never resolves

    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(),
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.user).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.serverError).toBe(false);
  });

  it('returns user data when authenticated', async () => {
    const mockUser = {
      name: 'Test User',
      email: 'test@example.com',
      avatar_url: 'https://example.com/avatar.png',
    };

    vi.mocked(authAPI.me).mockResolvedValue(mockUser);

    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.user).toEqual(mockUser);
    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.serverError).toBe(false);
  });

  it('returns null error for AuthenticationError (not logged in)', async () => {
    vi.mocked(authAPI.me).mockRejectedValue(new AuthenticationError());

    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.user).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.error).toBeNull(); // Auth errors are not shown as errors
    expect(result.current.serverError).toBe(false); // 401 is not a server error
  });

  it('returns error message for other errors', async () => {
    vi.useFakeTimers();
    try {
      vi.mocked(authAPI.me).mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useAuth(), {
        wrapper: createWrapper(),
      });

      // useAuth retries non-auth errors twice with exponential backoff (1s, 2s)
      // Advance past all retries
      await vi.advanceTimersByTimeAsync(10_000);

      expect(result.current.loading).toBe(false);
      expect(result.current.user).toBeNull();
      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.error).toBe('Network error');
      expect(result.current.serverError).toBe(true); // Non-auth error = server error
    } finally {
      vi.useRealTimers();
    }
  });

  it('provides refetch function', async () => {
    const mockUser = {
      name: 'Test User',
      email: 'test@example.com',
      avatar_url: 'https://example.com/avatar.png',
    };

    vi.mocked(authAPI.me).mockResolvedValue(mockUser);

    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(typeof result.current.refetch).toBe('function');
  });
});
