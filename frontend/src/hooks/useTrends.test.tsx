import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useTrends } from './useTrends';
import { trendsAPI } from '@/services/api';
import type { TrendsResponse } from '@/schemas/api';

vi.mock('@/services/api', () => ({
  trendsAPI: { get: vi.fn() },
}));

function makeResponse(overrides: Partial<TrendsResponse> = {}): TrendsResponse {
  return {
    computed_at: '2025-01-01T00:00:00Z',
    date_range: { start_date: '2025-01-01', end_date: '2025-01-31' },
    session_count: 5,
    repos_included: [],
    include_no_repo: true,
    providers_present: [],
    cards: {
      overview: null,
      tokens: null,
      activity: null,
      tools: null,
      utilization: null,
      agents_and_skills: null,
      top_sessions: null,
    },
    ...overrides,
  };
}

describe('useTrends', () => {
  beforeEach(() => {
    vi.mocked(trendsAPI.get).mockReset();
  });

  it('starts loading then transitions to data on mount', async () => {
    const response = makeResponse();
    vi.mocked(trendsAPI.get).mockResolvedValue(response);

    const { result } = renderHook(() => useTrends({ startDate: '2025-01-01' }));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual(response);
    expect(result.current.error).toBeNull();
    expect(trendsAPI.get).toHaveBeenCalledWith({ startDate: '2025-01-01' });
  });

  it('captures error from rejected API', async () => {
    vi.mocked(trendsAPI.get).mockRejectedValue(new Error('boom'));

    const { result } = renderHook(() => useTrends());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('boom');
    expect(result.current.data).toBeNull();
  });

  it('wraps non-Error rejections in an Error', async () => {
    vi.mocked(trendsAPI.get).mockRejectedValue('string failure');

    const { result } = renderHook(() => useTrends());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('Failed to fetch trends');
  });

  it('refetch() with no args re-uses captured initial params', async () => {
    vi.mocked(trendsAPI.get).mockResolvedValue(makeResponse());
    const { result } = renderHook(() => useTrends({ startDate: '2025-01-01' }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.refetch();
    expect(trendsAPI.get).toHaveBeenLastCalledWith({ startDate: '2025-01-01' });
  });

  it('refetch(newParams) updates params and uses them', async () => {
    vi.mocked(trendsAPI.get).mockResolvedValue(makeResponse());
    const { result } = renderHook(() => useTrends({ startDate: '2025-01-01' }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.refetch({ startDate: '2025-02-01', endDate: '2025-02-28' });
    expect(trendsAPI.get).toHaveBeenLastCalledWith({
      startDate: '2025-02-01',
      endDate: '2025-02-28',
    });
  });

  // CF-424: provider filter is forwarded verbatim to the trends API client.
  it('refetch with providers forwards the array to trendsAPI.get', async () => {
    vi.mocked(trendsAPI.get).mockResolvedValue(makeResponse());
    const { result } = renderHook(() => useTrends({ startDate: '2025-01-01' }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.refetch({ startDate: '2025-01-01', providers: ['claude-code'] });
    expect(trendsAPI.get).toHaveBeenLastCalledWith({
      startDate: '2025-01-01',
      providers: ['claude-code'],
    });
  });
});
