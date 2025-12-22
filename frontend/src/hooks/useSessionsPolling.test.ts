import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useSessionsPolling } from './useSessionsPolling';
import type { SessionListView } from '@/services/api';
import type { Session } from '@/types';

// Mock useSmartPolling since we test it separately
vi.mock('./useSmartPolling', () => ({
  useSmartPolling: vi.fn(),
}));

// Mock the API
vi.mock('@/services/api', () => ({
  sessionsAPI: {
    listWithETag: vi.fn(),
  },
}));

import { useSmartPolling } from './useSmartPolling';
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
    share_token: null,
    shared_by_email: null,
  },
];

// Helper to get the captured fetch function from the mock
function getCapturedFetchFn(): (() => Promise<unknown>) | undefined {
  const calls = vi.mocked(useSmartPolling).mock.calls;
  if (calls.length === 0) return undefined;
  const lastCall = calls[calls.length - 1];
  if (!lastCall) return undefined;
  // The first argument is the fetch function
  const fetchFn = lastCall[0];
  if (typeof fetchFn !== 'function') return undefined;
  return fetchFn;
}

describe('useSessionsPolling', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock for useSmartPolling
    vi.mocked(useSmartPolling).mockReturnValue({
      data: mockSessions,
      state: 'active',
      refetch: vi.fn(),
      loading: false,
      error: null,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns sessions from smart polling', () => {
    const { result } = renderHook(() => useSessionsPolling());

    expect(result.current.sessions).toEqual(mockSessions);
    expect(result.current.pollingState).toBe('active');
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('returns empty array when no data', () => {
    vi.mocked(useSmartPolling).mockReturnValue({
      data: null,
      state: 'active',
      refetch: vi.fn(),
      loading: true,
      error: null,
    });

    const { result } = renderHook(() => useSessionsPolling());

    expect(result.current.sessions).toEqual([]);
  });

  it('passes enabled option to smart polling', () => {
    renderHook(() => useSessionsPolling('owned', false));

    expect(useSmartPolling).toHaveBeenCalledWith(
      expect.any(Function),
      { enabled: false, resetKey: 'owned' }
    );
  });

  it('uses view parameter in API call', async () => {
    vi.mocked(sessionsAPI.listWithETag).mockResolvedValue({
      data: mockSessions,
      etag: '"abc123"',
    });

    renderHook(() => useSessionsPolling('shared'));

    // Get the captured fetch function
    const fetchFn = getCapturedFetchFn();
    expect(fetchFn).toBeDefined();
    await fetchFn!();

    expect(sessionsAPI.listWithETag).toHaveBeenCalledWith('shared', null);
  });

  it('tracks and sends ETag on subsequent requests', async () => {
    vi.mocked(sessionsAPI.listWithETag).mockResolvedValue({
      data: mockSessions,
      etag: '"etag-value"',
    });

    renderHook(() => useSessionsPolling());

    const fetchFn = getCapturedFetchFn();
    expect(fetchFn).toBeDefined();

    // First call - no etag
    await fetchFn!();
    expect(sessionsAPI.listWithETag).toHaveBeenCalledWith('owned', null);

    // Second call - should include etag
    await fetchFn!();
    expect(sessionsAPI.listWithETag).toHaveBeenLastCalledWith('owned', '"etag-value"');
  });

  it('returns null when API returns null data (304)', async () => {
    vi.mocked(sessionsAPI.listWithETag).mockResolvedValue({
      data: null,
      etag: '"same-etag"',
    });

    renderHook(() => useSessionsPolling());

    const fetchFn = getCapturedFetchFn();
    expect(fetchFn).toBeDefined();
    const result = await fetchFn!();

    expect(result).toBeNull();
  });

  it('provides refetch function from smart polling', () => {
    const mockRefetch = vi.fn();
    vi.mocked(useSmartPolling).mockReturnValue({
      data: mockSessions,
      state: 'active',
      refetch: mockRefetch,
      loading: false,
      error: null,
    });

    const { result } = renderHook(() => useSessionsPolling());

    expect(result.current.refetch).toBe(mockRefetch);
  });

  it('forwards error from smart polling', () => {
    const error = new Error('Test error');
    vi.mocked(useSmartPolling).mockReturnValue({
      data: null,
      state: 'active',
      refetch: vi.fn(),
      loading: false,
      error,
    });

    const { result } = renderHook(() => useSessionsPolling());

    expect(result.current.error).toBe(error);
  });

  it('creates new fetch function when view changes', () => {
    const { rerender } = renderHook<
      ReturnType<typeof useSessionsPolling>,
      { view: SessionListView }
    >(
      ({ view }) => useSessionsPolling(view),
      { initialProps: { view: 'owned' } }
    );

    const callCountAfterFirst = vi.mocked(useSmartPolling).mock.calls.length;
    const firstFetchFn = getCapturedFetchFn();

    rerender({ view: 'shared' });

    // Should have been called again with a new fetch function
    expect(vi.mocked(useSmartPolling).mock.calls.length).toBeGreaterThan(callCountAfterFirst);
    const secondFetchFn = getCapturedFetchFn();
    expect(secondFetchFn).not.toBe(firstFetchFn);
  });
});
