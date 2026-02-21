import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { APIError, NetworkError, AuthenticationError, analyticsAPI, trendsAPI } from './api';

describe('APIError', () => {
  it('should create APIError with correct properties', () => {
    const error = new APIError('Test error', 500, 'Internal Server Error', { detail: 'test' });

    expect(error.message).toBe('Test error');
    expect(error.status).toBe(500);
    expect(error.statusText).toBe('Internal Server Error');
    expect(error.data).toEqual({ detail: 'test' });
    expect(error.name).toBe('APIError');
  });

  it('should extract backend error message from data.error', () => {
    const error = new APIError('Request failed: Bad Request', 400, 'Bad Request', {
      error: 'Custom title exceeds maximum length of 255 characters',
    });

    expect(error.message).toBe('Custom title exceeds maximum length of 255 characters');
    expect(error.status).toBe(400);
    expect(error.data).toEqual({ error: 'Custom title exceeds maximum length of 255 characters' });
  });

  it('should fallback to generic message when data.error is not present', () => {
    const error = new APIError('Request failed: Internal Server Error', 500, 'Internal Server Error', {
      details: 'something went wrong',
    });

    expect(error.message).toBe('Request failed: Internal Server Error');
  });

  it('should fallback to generic message when data is not an object', () => {
    const error = new APIError('Request failed: Bad Request', 400, 'Bad Request', 'plain text error');

    expect(error.message).toBe('Request failed: Bad Request');
  });
});

describe('NetworkError', () => {
  it('should create NetworkError with correct properties', () => {
    const error = new NetworkError('Network failed');

    expect(error.message).toBe('Network failed');
    expect(error.name).toBe('NetworkError');
  });
});

describe('AuthenticationError', () => {
  it('should create AuthenticationError with default message', () => {
    const error = new AuthenticationError();

    expect(error.message).toBe('Authentication required');
    expect(error.status).toBe(401);
    expect(error.statusText).toBe('Unauthorized');
    expect(error.name).toBe('AuthenticationError');
  });

  it('should create AuthenticationError with custom message', () => {
    const error = new AuthenticationError('Custom auth error');

    expect(error.message).toBe('Custom auth error');
    expect(error.status).toBe(401);
  });
});

// Minimal valid SessionAnalytics response for Zod validation
const validAnalyticsResponse = {
  computed_at: '2024-01-01T00:00:00Z',
  computed_lines: 100,
  tokens: { input: 1000, output: 500, cache_creation: 0, cache_read: 0 },
  cost: { estimated_usd: '0.10' },
  compaction: { auto: 0, manual: 0 },
  cards: null,
};

// Minimal valid TrendsResponse for Zod validation
const validTrendsResponse = {
  computed_at: '2024-01-01T00:00:00Z',
  date_range: { start_date: '2024-01-01', end_date: '2024-01-02' },
  session_count: 5,
  repos_included: [],
  include_no_repo: true,
  cards: {
    overview: null,
    tokens: null,
    activity: null,
    tools: null,
    utilization: null,
    agents_and_skills: null,
  },
};

/** Build a fake Response object for fetch mock. */
function fakeResponse(props: Record<string, unknown>): Response {
  // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
  return props as unknown as Response;
}

/** Set up fetch spy and return the spy. Also mocks window.location. */
function setupFetchSpy() {
  const spy = vi.spyOn(global, 'fetch');
  Object.defineProperty(window, 'location', {
    writable: true,
    value: { href: '/dashboard' },
  });
  return spy;
}

describe('fetchRaw (via trendsAPI.get)', () => {
  let fetchSpy: ReturnType<typeof setupFetchSpy>;
  const originalLocation = window.location;

  beforeEach(() => {
    fetchSpy = setupFetchSpy();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(window, 'location', {
      writable: true,
      value: originalLocation,
    });
  });

  it('includes credentials: include in fetch call', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validTrendsResponse),
    }));

    await trendsAPI.get();

    expect(fetchSpy).toHaveBeenCalledOnce();
    const call = fetchSpy.mock.calls[0];
    expect(call?.[1]?.credentials).toBe('include');
  });

  it('401 response throws AuthenticationError and redirects', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
    }));

    await expect(trendsAPI.get()).rejects.toThrow(AuthenticationError);
    expect(window.location.href).toBe('/');
  });

  it('non-ok response with JSON body throws APIError with parsed data', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve({ error: 'server broke' }),
    }));

    try {
      await trendsAPI.get();
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(APIError);
      const apiErr = err instanceof APIError ? err : undefined;
      expect(apiErr?.status).toBe(500);
      expect(apiErr?.data).toEqual({ error: 'server broke' });
      expect(apiErr?.message).toBe('server broke');
    }
  });

  it('non-ok response with text body throws APIError with text data', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: false,
      status: 502,
      statusText: 'Bad Gateway',
      json: () => Promise.reject(new Error('not json')),
      text: () => Promise.resolve('upstream timeout'),
    }));

    try {
      await trendsAPI.get();
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(APIError);
      const apiErr = err instanceof APIError ? err : undefined;
      expect(apiErr?.status).toBe(502);
      expect(apiErr?.data).toBe('upstream timeout');
    }
  });
});

describe('analyticsAPI.regenerateSmartRecap', () => {
  let fetchSpy: ReturnType<typeof setupFetchSpy>;

  beforeEach(() => {
    fetchSpy = setupFetchSpy();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('sends POST to correct endpoint', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validAnalyticsResponse),
    }));

    await analyticsAPI.regenerateSmartRecap('session-123');

    expect(fetchSpy).toHaveBeenCalledOnce();
    const call = fetchSpy.mock.calls[0];
    expect(call?.[0]).toBe('/api/v1/sessions/session-123/analytics/smart-recap/regenerate');
    expect(call?.[1]?.method).toBe('POST');
  });

  it('returns validated SessionAnalytics', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validAnalyticsResponse),
    }));

    const result = await analyticsAPI.regenerateSmartRecap('session-123');
    expect(result.computed_at).toBe('2024-01-01T00:00:00Z');
    expect(result.computed_lines).toBe(100);
  });
});

describe('trendsAPI.get', () => {
  let fetchSpy: ReturnType<typeof setupFetchSpy>;

  beforeEach(() => {
    fetchSpy = setupFetchSpy();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('no params: URL has only tz_offset', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validTrendsResponse),
    }));

    await trendsAPI.get();

    const url = String(fetchSpy.mock.calls[0]?.[0] ?? '');
    const parsed = new URL(url, 'http://localhost');
    expect(parsed.pathname).toBe('/api/v1/trends');
    expect(parsed.searchParams.has('tz_offset')).toBe(true);
    expect(parsed.searchParams.has('start_ts')).toBe(false);
    expect(parsed.searchParams.has('end_ts')).toBe(false);
  });

  it('with startDate/endDate: converts to epoch, endDate adds 86400', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validTrendsResponse),
    }));

    await trendsAPI.get({ startDate: '2024-01-15', endDate: '2024-01-20' });

    const url = String(fetchSpy.mock.calls[0]?.[0] ?? '');
    const parsed = new URL(url, 'http://localhost');

    const startTs = Number(parsed.searchParams.get('start_ts'));
    const endTs = Number(parsed.searchParams.get('end_ts'));

    // startDate: 2024-01-15 at local midnight
    const expectedStart = Math.floor(new Date(2024, 0, 15).getTime() / 1000);
    expect(startTs).toBe(expectedStart);

    // endDate: 2024-01-20 at local midnight + 86400 (exclusive end)
    const expectedEnd = Math.floor(new Date(2024, 0, 20).getTime() / 1000) + 86400;
    expect(endTs).toBe(expectedEnd);
  });

  it('with repos: joins with commas', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validTrendsResponse),
    }));

    await trendsAPI.get({ repos: ['repo-a', 'repo-b'] });

    const url = String(fetchSpy.mock.calls[0]?.[0] ?? '');
    const parsed = new URL(url, 'http://localhost');
    expect(parsed.searchParams.get('repos')).toBe('repo-a,repo-b');
  });

  it('with includeNoRepo: sets param', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validTrendsResponse),
    }));

    await trendsAPI.get({ includeNoRepo: true });

    const url = String(fetchSpy.mock.calls[0]?.[0] ?? '');
    const parsed = new URL(url, 'http://localhost');
    expect(parsed.searchParams.get('include_no_repo')).toBe('true');
  });

  it('empty repos array is not included', async () => {
    fetchSpy.mockResolvedValue(fakeResponse({
      ok: true,
      status: 200,
      json: () => Promise.resolve(validTrendsResponse),
    }));

    await trendsAPI.get({ repos: [] });

    const url = String(fetchSpy.mock.calls[0]?.[0] ?? '');
    const parsed = new URL(url, 'http://localhost');
    expect(parsed.searchParams.has('repos')).toBe(false);
  });
});
