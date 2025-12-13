import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useSession } from './useSession';
import type { ReactNode } from 'react';
import type { SessionDetail } from '@/types';

// Mock react-router-dom
const mockNavigate = vi.fn();
vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}));

// Mock the API module
vi.mock('@/services/api', () => ({
  sessionsAPI: {
    get: vi.fn(),
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
const mockSession: SessionDetail = {
  id: '1',
  external_id: 'ext-1',
  summary: 'Test session',
  first_user_message: 'Hello',
  first_seen: '2025-01-01T10:00:00Z',
  cwd: '/test/path',
  transcript_path: '/test/transcript.jsonl',
  git_info: {
    repo_url: 'https://github.com/test/repo',
    branch: 'main',
    commit_sha: 'abc123',
  },
  last_sync_at: '2025-01-01T12:00:00Z',
  files: [
    {
      file_name: 'transcript.jsonl',
      file_type: 'transcript',
      last_synced_line: 100,
      updated_at: '2025-01-01T12:00:00Z',
    },
  ],
};

describe('useSession', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns loading state initially', () => {
    vi.mocked(sessionsAPI.get).mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useSession('session-1'), {
      wrapper: createWrapper(),
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.session).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('returns session data on success', async () => {
    vi.mocked(sessionsAPI.get).mockResolvedValue(mockSession);

    const { result } = renderHook(() => useSession('session-1'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toEqual(mockSession);
    expect(result.current.error).toBeNull();
    expect(sessionsAPI.get).toHaveBeenCalledWith('session-1');
  });

  it('does not fetch when sessionId is undefined', async () => {
    const { result } = renderHook(() => useSession(undefined), {
      wrapper: createWrapper(),
    });

    // Should not be loading since query is disabled
    expect(result.current.loading).toBe(false);
    expect(result.current.session).toBeNull();
    expect(sessionsAPI.get).not.toHaveBeenCalled();
  });

  it('navigates to home on AuthenticationError', async () => {
    vi.mocked(sessionsAPI.get).mockRejectedValue(new AuthenticationError());

    renderHook(() => useSession('session-1'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });
  });

  it('returns null error for AuthenticationError', async () => {
    vi.mocked(sessionsAPI.get).mockRejectedValue(new AuthenticationError());

    const { result } = renderHook(() => useSession('session-1'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBeNull();
  });

  it('returns error message for other errors', async () => {
    vi.mocked(sessionsAPI.get).mockRejectedValue(new Error('Session not found'));

    const { result } = renderHook(() => useSession('session-1'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('Session not found');
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('provides refetch function', async () => {
    vi.mocked(sessionsAPI.get).mockResolvedValue(mockSession);

    const { result } = renderHook(() => useSession('session-1'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(typeof result.current.refetch).toBe('function');
  });

  it('refetches when sessionId changes', async () => {
    vi.mocked(sessionsAPI.get).mockResolvedValue(mockSession);

    const { result, rerender } = renderHook(
      ({ sessionId }) => useSession(sessionId),
      {
        wrapper: createWrapper(),
        initialProps: { sessionId: 'session-1' },
      }
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(sessionsAPI.get).toHaveBeenCalledWith('session-1');

    // Change sessionId
    rerender({ sessionId: 'session-2' });

    await waitFor(() => {
      expect(sessionsAPI.get).toHaveBeenCalledWith('session-2');
    });
  });
});
