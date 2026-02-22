import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { AppConfigProvider } from './AppConfigContext';
import { fetchConfigWithRetry } from './fetchAppConfig';
import { useAppConfig } from '@/hooks/useAppConfig';
import type { ReactNode } from 'react';

function createWrapper() {
  return ({ children }: { children: ReactNode }) => (
    <AppConfigProvider>{children}</AppConfigProvider>
  );
}

function makeFakeResponse(ok: boolean, data?: Record<string, unknown>): Response {
  // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
  return { ok, status: ok ? 200 : 500, json: async () => data ?? {} } as Response;
}

function mockFetchResponse(ok: boolean, data?: Record<string, unknown>) {
  return vi.spyOn(global, 'fetch').mockResolvedValue(makeFakeResponse(ok, data));
}

describe('AppConfigContext', () => {
  beforeEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('defaults sharesEnabled to false before fetch resolves', () => {
    vi.spyOn(global, 'fetch').mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    expect(result.current.sharesEnabled).toBe(false);
  });

  it('sets sharesEnabled to true from server response', async () => {
    mockFetchResponse(true, {
      providers: [],
      features: { shares_enabled: true },
    });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.sharesEnabled).toBe(true);
    });
  });

  it('sets sharesEnabled to false when server reports disabled', async () => {
    mockFetchResponse(true, {
      providers: [],
      features: { shares_enabled: false },
    });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.sharesEnabled).toBe(false);
    });
  });

  it('defaults saasFooterEnabled to false before fetch resolves', () => {
    vi.spyOn(global, 'fetch').mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    expect(result.current.saasFooterEnabled).toBe(false);
  });

  it('sets saasFooterEnabled to true when server reports enabled', async () => {
    mockFetchResponse(true, {
      providers: [],
      features: { shares_enabled: true, saas_footer_enabled: true },
    });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.saasFooterEnabled).toBe(true);
    });
  });

  it('defaults saasFooterEnabled to false when features field is missing', async () => {
    mockFetchResponse(true, { providers: [] });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.saasFooterEnabled).toBe(false);
  });

  it('defaults saasTermlyEnabled to false before fetch resolves', () => {
    vi.spyOn(global, 'fetch').mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    expect(result.current.saasTermlyEnabled).toBe(false);
  });

  it('sets saasTermlyEnabled to true when server reports enabled', async () => {
    mockFetchResponse(true, {
      providers: [],
      features: { shares_enabled: true, saas_termly_enabled: true },
    });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.saasTermlyEnabled).toBe(true);
    });
  });

  it('defaults saasTermlyEnabled to false when features field is missing', async () => {
    mockFetchResponse(true, { providers: [] });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.saasTermlyEnabled).toBe(false);
  });

  it('reads supportEmail from server response', async () => {
    mockFetchResponse(true, {
      providers: [],
      features: { shares_enabled: true, saas_footer_enabled: true, support_email: 'help@example.com' },
    });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.supportEmail).toBe('help@example.com');
    });
  });

  it('defaults supportEmail to empty string before fetch resolves', () => {
    vi.spyOn(global, 'fetch').mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    expect(result.current.supportEmail).toBe('');
  });

  it('defaults supportEmail to empty string when features field is missing', async () => {
    mockFetchResponse(true, { providers: [] });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.supportEmail).toBe('');
  });
});

// Test retry logic directly on the exported function (avoids fake timer + waitFor conflicts)
describe('fetchConfigWithRetry', () => {
  beforeEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('retries on fetch failure and defaults to false', async () => {
    vi.useFakeTimers();
    const fetchSpy = vi.spyOn(global, 'fetch').mockRejectedValue(new Error('Network error'));

    const promise = fetchConfigWithRetry();

    // Advance through retry delays: 1s, 2s (third attempt has no delay after it)
    await vi.advanceTimersByTimeAsync(1000);
    await vi.advanceTimersByTimeAsync(2000);

    const result = await promise;

    expect(fetchSpy).toHaveBeenCalledTimes(3);
    expect(result.saasFooterEnabled).toBe(false);
    expect(result.saasTermlyEnabled).toBe(false);
    expect(result.sharesEnabled).toBe(false);
  });

  it('retries on non-ok response and defaults to false', async () => {
    vi.useFakeTimers();
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(makeFakeResponse(false));

    const promise = fetchConfigWithRetry();

    await vi.advanceTimersByTimeAsync(1000);
    await vi.advanceTimersByTimeAsync(2000);

    const result = await promise;

    expect(fetchSpy).toHaveBeenCalledTimes(3);
    expect(result.saasFooterEnabled).toBe(false);
    expect(result.saasTermlyEnabled).toBe(false);
  });

  it('succeeds on retry after initial failure', async () => {
    vi.useFakeTimers();
    const fetchSpy = vi
      .spyOn(global, 'fetch')
      .mockRejectedValueOnce(new Error('Network error'))
      .mockResolvedValueOnce(
        makeFakeResponse(true, {
          providers: [],
          features: { shares_enabled: true, saas_footer_enabled: true, saas_termly_enabled: true },
        }),
      );

    const promise = fetchConfigWithRetry();

    // Advance past first retry delay (1s)
    await vi.advanceTimersByTimeAsync(1000);

    const result = await promise;

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    expect(result.saasFooterEnabled).toBe(true);
    expect(result.saasTermlyEnabled).toBe(true);
  });
});
