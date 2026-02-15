import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useSessionsFetch } from './useSessionsFetch';
import type { SessionFilters } from './useSessionFilters';
import type { SessionListResponse } from '@/schemas/api';

// Mock the API
vi.mock('@/services/api', () => ({
  sessionsAPI: {
    list: vi.fn(),
  },
}));

import { sessionsAPI } from '@/services/api';

const defaultFilters: SessionFilters = {
  repos: [],
  branches: [],
  owners: [],
  query: '',
  page: 1,
};

const mockResponse: SessionListResponse = {
  sessions: [
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
      git_repo: 'test/repo',
      git_branch: 'main',
      is_owner: true,
      access_type: 'owner',
      shared_by_email: null,
    },
  ],
  total: 1,
  page: 1,
  page_size: 50,
  filter_options: {
    repos: [{ value: 'test/repo', count: 1 }],
    branches: [{ value: 'main', count: 1 }],
    owners: [{ value: 'user@test.com', count: 1 }],
    total: 1,
  },
};

const emptyResponse: SessionListResponse = {
  sessions: [],
  total: 0,
  page: 1,
  page_size: 50,
  filter_options: { repos: [], branches: [], owners: [], total: 0 },
};

describe('useSessionsFetch', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(sessionsAPI.list).mockResolvedValue(mockResponse);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches sessions on mount with default filters', async () => {
    const { result } = renderHook(() => useSessionsFetch(defaultFilters));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual(mockResponse.sessions);
    expect(result.current.total).toBe(1);
    expect(result.current.filterOptions).toEqual(mockResponse.filter_options);
    expect(sessionsAPI.list).toHaveBeenCalledTimes(1);
  });

  it('returns empty results', async () => {
    vi.mocked(sessionsAPI.list).mockResolvedValue(emptyResponse);

    const { result } = renderHook(() => useSessionsFetch(defaultFilters));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sessions).toEqual([]);
    expect(result.current.total).toBe(0);
  });

  it('passes filter params to API', async () => {
    const filters: SessionFilters = {
      repos: ['confab-web'],
      branches: ['main'],
      owners: [],
      query: '',
      page: 2,
    };

    const { result } = renderHook(() => useSessionsFetch(filters));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(sessionsAPI.list).toHaveBeenCalledWith({
      repo: 'confab-web',
      branch: 'main',
      page: '2',
    });
  });

  it('handles fetch errors', async () => {
    const error = new Error('Network error');
    vi.mocked(sessionsAPI.list).mockRejectedValue(error);

    const { result } = renderHook(() => useSessionsFetch(defaultFilters));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(error);
    expect(result.current.sessions).toEqual([]);
  });

  it('refetch fetches sessions again', async () => {
    const { result } = renderHook(() => useSessionsFetch(defaultFilters));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(sessionsAPI.list).toHaveBeenCalledTimes(1);

    await act(async () => {
      await result.current.refetch();
    });

    expect(sessionsAPI.list).toHaveBeenCalledTimes(2);
  });

  it('clears error on successful refetch', async () => {
    vi.mocked(sessionsAPI.list).mockRejectedValueOnce(new Error('fail'));

    const { result } = renderHook(() => useSessionsFetch(defaultFilters));

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    vi.mocked(sessionsAPI.list).mockResolvedValue(mockResponse);

    await act(async () => {
      await result.current.refetch();
    });

    expect(result.current.error).toBeNull();
    expect(result.current.sessions).toEqual(mockResponse.sessions);
  });
});
