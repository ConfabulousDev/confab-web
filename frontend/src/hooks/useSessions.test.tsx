import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useSessions } from './useSessions';
import type { ReactNode } from 'react';
import type { Session } from '@/types';

// Mock the API module
vi.mock('@/services/api', () => ({
  sessionsAPI: {
    list: vi.fn(),
  },
  AuthenticationError: class AuthenticationError extends Error {
    constructor(message = 'Authentication required') {
      super(message);
      this.name = 'AuthenticationError';
    }
  },
}));

import { sessionsAPI, AuthenticationError } from '@/services/api';

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

// Sample session data
const mockSessions: Session[] = [
  {
    id: '1',
    external_id: 'ext-1',
    first_seen: '2025-01-01T10:00:00Z',
    file_count: 2,
    last_sync_time: '2025-01-01T12:00:00Z',
    summary: 'Test session 1',
    first_user_message: 'Hello',
    session_type: 'cli',
    total_lines: 100,
    git_repo: 'https://github.com/test/repo',
    git_branch: 'main',
    is_owner: true,
    access_type: 'owner',
    share_token: null,
    shared_by_email: null,
  },
  {
    id: '2',
    external_id: 'ext-2',
    first_seen: '2025-01-02T10:00:00Z',
    file_count: 1,
    last_sync_time: null,
    summary: null,
    first_user_message: 'Hi there',
    session_type: 'cli',
    total_lines: 50,
    git_repo: null,
    git_branch: null,
    is_owner: true,
    access_type: 'owner',
    share_token: null,
    shared_by_email: null,
  },
];

describe('useSessions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns loading state initially', () => {
    vi.mocked(sessionsAPI.list).mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.sessions).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('returns sessions data on success', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockSessions);

    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual(mockSessions);
    expect(result.current.error).toBeNull();
  });

  it('returns empty array when no sessions', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue([]);

    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('passes view parameter to API', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockSessions);

    renderHook(() => useSessions('shared'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(sessionsAPI.list).toHaveBeenCalledWith('shared');
    });
  });

  it('defaults view to owned', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockSessions);

    renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(sessionsAPI.list).toHaveBeenCalledWith('owned');
    });
  });

  it('returns null error for AuthenticationError', async () => {
    vi.mocked(sessionsAPI.list).mockRejectedValue(new AuthenticationError());

    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('returns error message for other errors', async () => {
    vi.mocked(sessionsAPI.list).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual([]);
    expect(result.current.error).toBe('Network error');
  });

  it('provides refetch function', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockSessions);

    const { result } = renderHook(() => useSessions(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(typeof result.current.refetch).toBe('function');
  });
});
