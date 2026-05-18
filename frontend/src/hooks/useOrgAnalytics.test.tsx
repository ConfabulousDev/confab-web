import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useOrgAnalytics } from './useOrgAnalytics';
import { orgAnalyticsAPI } from '@/services/api';
import type { OrgAnalyticsResponse } from '@/schemas/api';

vi.mock('@/services/api', () => ({
  orgAnalyticsAPI: { get: vi.fn() },
}));

function makeResponse(): OrgAnalyticsResponse {
  return {
    computed_at: '2025-01-01T00:00:00Z',
    date_range: { start_date: '2025-01-01', end_date: '2025-01-31' },
    providers_present: [],
    users: [],
  };
}

describe('useOrgAnalytics', () => {
  beforeEach(() => {
    vi.mocked(orgAnalyticsAPI.get).mockReset();
  });

  it('fetches on mount with initial params', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockResolvedValue(makeResponse());

    const { result } = renderHook(() =>
      useOrgAnalytics({ startDate: '2025-01-01', endDate: '2025-01-31' })
    );

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(orgAnalyticsAPI.get).toHaveBeenCalledWith({
      startDate: '2025-01-01',
      endDate: '2025-01-31',
    });
    expect(result.current.data).not.toBeNull();
  });

  it('surfaces error on rejection', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockRejectedValue(new Error('nope'));

    const { result } = renderHook(() => useOrgAnalytics({}));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error?.message).toBe('nope');
    expect(result.current.data).toBeNull();
  });

  it('wraps non-Error rejections', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockRejectedValue({ code: 500 });

    const { result } = renderHook(() => useOrgAnalytics({}));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error?.message).toBe('Failed to fetch org analytics');
  });

  it('refetch(params) calls API with new params', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockResolvedValue(makeResponse());
    const { result } = renderHook(() => useOrgAnalytics({}));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.refetch({ startDate: '2025-03-01' });
    expect(orgAnalyticsAPI.get).toHaveBeenLastCalledWith({ startDate: '2025-03-01' });
  });

  it('round-trips a providers filter to the API call', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockResolvedValue(makeResponse());
    const { result } = renderHook(() =>
      useOrgAnalytics({ providers: ['codex'] })
    );
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(orgAnalyticsAPI.get).toHaveBeenCalledWith({ providers: ['codex'] });
  });

  it('skips initial fetch when enabled=false; fires once on enable', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockResolvedValue(makeResponse());

    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) =>
        useOrgAnalytics({ startDate: '2025-01-01' }, { enabled }),
      { initialProps: { enabled: false } }
    );

    expect(orgAnalyticsAPI.get).not.toHaveBeenCalled();
    expect(result.current.loading).toBe(false);

    rerender({ enabled: true });
    await waitFor(() => expect(orgAnalyticsAPI.get).toHaveBeenCalledTimes(1));
    expect(orgAnalyticsAPI.get).toHaveBeenCalledWith({ startDate: '2025-01-01' });
  });

  it('does not trigger a second fetch when initialParams prop changes after mount', async () => {
    vi.mocked(orgAnalyticsAPI.get).mockResolvedValue(makeResponse());
    const { rerender, result } = renderHook(
      ({ params }: { params: { startDate?: string } }) => useOrgAnalytics(params),
      { initialProps: { params: { startDate: '2025-01-01' } } }
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(orgAnalyticsAPI.get).toHaveBeenCalledTimes(1);

    rerender({ params: { startDate: '2025-02-01' } });
    rerender({ params: { startDate: '2025-03-01' } });

    expect(orgAnalyticsAPI.get).toHaveBeenCalledTimes(1);
    expect(orgAnalyticsAPI.get).toHaveBeenCalledWith({ startDate: '2025-01-01' });
  });

  it('on-enable fetch uses latest params, not the captured-at-mount params', async () => {
    // OrgPage mounts with `repos: []`, learns the repo list from /org/repos,
    // auto-selects, then flips `enabled` to true. The fire on enable must use
    // the post-auto-select params or `providers_present` collapses to no-repo
    // sessions only (reproducing the inconsistent dropdown bug).
    vi.mocked(orgAnalyticsAPI.get).mockResolvedValue(makeResponse());

    type Props = { params: { repos: string[] }; enabled: boolean };
    const initialProps: Props = { params: { repos: [] }, enabled: false };
    const { rerender } = renderHook(
      ({ params, enabled }: Props) => useOrgAnalytics(params, { enabled }),
      { initialProps }
    );

    expect(orgAnalyticsAPI.get).not.toHaveBeenCalled();

    rerender({ params: { repos: ['foo/bar', 'baz/qux'] }, enabled: false });
    rerender({ params: { repos: ['foo/bar', 'baz/qux'] }, enabled: true });

    await waitFor(() => expect(orgAnalyticsAPI.get).toHaveBeenCalledTimes(1));
    expect(orgAnalyticsAPI.get).toHaveBeenCalledWith({ repos: ['foo/bar', 'baz/qux'] });
  });
});
