import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useLoadSession } from './useLoadSession';
import type { SessionDetail } from '@/types';

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

describe('useLoadSession', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns loading state initially', () => {
    const fetchSession = vi.fn(() => new Promise<SessionDetail>(() => {}));

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    expect(result.current.loading).toBe(true);
    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('');
    expect(result.current.errorType).toBeNull();
  });

  it('returns session data on success', async () => {
    const fetchSession = vi.fn().mockResolvedValue(mockSession);

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toEqual(mockSession);
    expect(result.current.error).toBe('');
    expect(result.current.errorType).toBeNull();
  });

  it('handles 404 error with not_found type', async () => {
    const error = { status: 404 };
    const fetchSession = vi.fn().mockRejectedValue(error);

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('Session not found');
    expect(result.current.errorType).toBe('not_found');
  });

  it('handles 410 error with expired type', async () => {
    const error = { status: 410 };
    const fetchSession = vi.fn().mockRejectedValue(error);

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('This share link has expired');
    expect(result.current.errorType).toBe('expired');
  });

  it('handles 403 error with forbidden type', async () => {
    const error = { status: 403 };
    const fetchSession = vi.fn().mockRejectedValue(error);

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('You are not authorized to view this session');
    expect(result.current.errorType).toBe('forbidden');
  });

  it('handles 401 error with auth_required errorType', async () => {
    const error = { status: 401 };
    const fetchSession = vi.fn().mockRejectedValue(error);

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('Sign in to view this session');
    expect(result.current.errorType).toBe('auth_required');
  });

  it('handles generic errors', async () => {
    const fetchSession = vi.fn().mockRejectedValue(new Error('Network failure'));

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toBe('Network failure');
    expect(result.current.errorType).toBe('general');
  });

  it('provides setError function', async () => {
    const fetchSession = vi.fn().mockResolvedValue(mockSession);

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.setError('Custom error', 'forbidden');
    });

    expect(result.current.error).toBe('Custom error');
    expect(result.current.errorType).toBe('forbidden');
  });

  it('provides clearError function', async () => {
    const fetchSession = vi.fn().mockRejectedValue(new Error('Initial error'));

    const { result } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Initial error');

    act(() => {
      result.current.clearError();
    });

    expect(result.current.error).toBe('');
    expect(result.current.errorType).toBeNull();
  });

  it('refetches when deps change', async () => {
    const fetchSession = vi.fn().mockResolvedValue(mockSession);

    const { result, rerender } = renderHook(
      ({ sessionId }) =>
        useLoadSession({
          fetchSession,
          deps: [sessionId],
        }),
      { initialProps: { sessionId: '1' } }
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(fetchSession).toHaveBeenCalledTimes(1);

    // Change deps
    rerender({ sessionId: '2' });

    await waitFor(() => {
      expect(fetchSession).toHaveBeenCalledTimes(2);
    });
  });

  it('cancels fetch on unmount', async () => {
    let resolvePromise: (value: SessionDetail) => void;
    const fetchSession = vi.fn().mockImplementation(
      () => new Promise<SessionDetail>((resolve) => {
        resolvePromise = resolve;
      })
    );

    const { unmount } = renderHook(() =>
      useLoadSession({ fetchSession })
    );

    // Unmount before resolving
    unmount();

    // Resolve after unmount - should not cause state updates
    resolvePromise!(mockSession);

    // No error should be thrown
  });
});
