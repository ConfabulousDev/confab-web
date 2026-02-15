import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { AppConfigProvider } from './AppConfigContext';
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
    vi.restoreAllMocks();
  });

  it('defaults sharesEnabled to true before fetch resolves', () => {
    vi.spyOn(global, 'fetch').mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    expect(result.current.sharesEnabled).toBe(true);
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

  it('defaults to true when fetch fails', async () => {
    vi.spyOn(global, 'fetch').mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    // Wait for fetch to complete (error path)
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.sharesEnabled).toBe(true);
  });

  it('defaults to true when response is not ok', async () => {
    mockFetchResponse(false);

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.sharesEnabled).toBe(true);
  });

  it('defaults to true when features field is missing', async () => {
    mockFetchResponse(true, { providers: [] });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.sharesEnabled).toBe(true);
  });

  it('defaults footerEnabled to true before fetch resolves', () => {
    vi.spyOn(global, 'fetch').mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    expect(result.current.footerEnabled).toBe(true);
  });

  it('sets footerEnabled to false when server reports disabled', async () => {
    mockFetchResponse(true, {
      providers: [],
      features: { shares_enabled: true, footer_enabled: false },
    });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.footerEnabled).toBe(false);
    });
  });

  it('defaults footerEnabled to true when features field is missing', async () => {
    mockFetchResponse(true, { providers: [] });

    const { result } = renderHook(() => useAppConfig(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled();
    });

    expect(result.current.footerEnabled).toBe(true);
  });
});
