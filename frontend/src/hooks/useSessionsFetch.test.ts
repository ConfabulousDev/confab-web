import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useSessionsFetch } from './useSessionsFetch';
import type { Session } from '@/types';

// Mock the API
vi.mock('@/services/api', () => ({
  sessionsAPI: {
    list: vi.fn(),
  },
}));

import { sessionsAPI } from '@/services/api';

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
    shared_by_email: null,
  },
];

describe('useSessionsFetch', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockSessions);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches sessions on mount', async () => {
    const { result } = renderHook(() => useSessionsFetch());

    // Initially loading
    expect(result.current.loading).toBe(true);
    expect(result.current.sessions).toEqual([]);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual(mockSessions);
    expect(result.current.error).toBeNull();
    expect(sessionsAPI.list).toHaveBeenCalledTimes(1);
  });

  it('returns empty array when no data', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue([]);

    const { result } = renderHook(() => useSessionsFetch());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual([]);
  });

  it('does not fetch when enabled=false', async () => {
    const { result } = renderHook(() => useSessionsFetch(false));

    // Should not start loading
    expect(result.current.loading).toBe(false);
    expect(result.current.sessions).toEqual([]);
    expect(sessionsAPI.list).not.toHaveBeenCalled();
  });

  it('handles fetch errors', async () => {
    const error = new Error('Network error');
    vi.mocked(sessionsAPI.list).mockRejectedValue(error);

    const { result } = renderHook(() => useSessionsFetch());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(error);
    expect(result.current.sessions).toEqual([]);
  });

  it('refetch fetches sessions again', async () => {
    const { result } = renderHook(() => useSessionsFetch());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(sessionsAPI.list).toHaveBeenCalledTimes(1);

    // Trigger refetch with updated data
    vi.mocked(sessionsAPI.list).mockResolvedValue([...mockSessions, ...mockSessions]);

    await act(async () => {
      await result.current.refetch();
    });

    expect(sessionsAPI.list).toHaveBeenCalledTimes(2);
    expect(result.current.sessions).toHaveLength(2);
  });

  it('clears error on successful refetch', async () => {
    vi.mocked(sessionsAPI.list).mockRejectedValueOnce(new Error('fail'));

    const { result } = renderHook(() => useSessionsFetch());

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    // Refetch succeeds
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockSessions);

    await act(async () => {
      await result.current.refetch();
    });

    expect(result.current.error).toBeNull();
    expect(result.current.sessions).toEqual(mockSessions);
  });
});
