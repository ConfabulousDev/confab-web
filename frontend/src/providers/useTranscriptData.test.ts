// et0r: useTranscriptData — cache reuse for revisited subagent tabs and
// late-file (404) recovery for subagent transcripts not yet synced.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { APIError } from '@/services/api';
import { getAdapter } from './registry';
import type { OpaqueAdapter } from './types';
import { useTranscriptData } from './useTranscriptData';

vi.mock('@/hooks/useVisibility', () => ({ useVisibility: () => true }));

const POLL_MS = 15_000;

// Real adapter shape (Claude's normalize is identity) with the fetchers mocked.
function makeAdapter(overrides: Partial<OpaqueAdapter> = {}): OpaqueAdapter {
  return {
    ...getAdapter('claude-code'),
    fetchInitial: vi.fn().mockResolvedValue({ items: ['a'], raw: ['a'], totalLines: 1 }),
    fetchIncremental: vi.fn().mockResolvedValue({ newItems: [], newRaw: [], newTotalLineCount: 1 }),
    ...overrides,
  };
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.spyOn(console, 'error').mockImplementation(() => {});
  vi.spyOn(console, 'warn').mockImplementation(() => {});
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('useTranscriptData', () => {
  it('skips the cache on initial load by default', async () => {
    const adapter = makeAdapter();
    renderHook(() => useTranscriptData(adapter, 's1', 'transcript.jsonl', undefined));
    await flush();
    expect(adapter.fetchInitial).toHaveBeenCalledWith('s1', 'transcript.jsonl', true);
  });

  it('reuses the cache on initial load when preferCache is set', async () => {
    const adapter = makeAdapter();
    renderHook(() => useTranscriptData(adapter, 's1', 'agent-a1.jsonl', undefined, { preferCache: true }));
    await flush();
    expect(adapter.fetchInitial).toHaveBeenCalledWith('s1', 'agent-a1.jsonl', false);
  });

  it('does not fetch when fileName is undefined', async () => {
    const adapter = makeAdapter();
    renderHook(() => useTranscriptData(adapter, 's1', undefined, undefined));
    await flush();
    expect(adapter.fetchInitial).not.toHaveBeenCalled();
  });

  it('flags notFound on a 404 and recovers by retrying the initial fetch on the poll', async () => {
    const fetchInitial = vi
      .fn()
      .mockRejectedValueOnce(new APIError('File not found', 404, 'Not Found'))
      .mockResolvedValueOnce({ items: ['x', 'y'], raw: ['x', 'y'], totalLines: 2 });
    const adapter = makeAdapter({ fetchInitial });
    const { result } = renderHook(() =>
      useTranscriptData(adapter, 's1', 'agent-a1.jsonl', undefined, { preferCache: true }),
    );
    await flush();
    expect(result.current.notFound).toBe(true);
    expect(result.current.loading).toBe(false);

    await act(async () => {
      vi.advanceTimersByTime(POLL_MS);
    });
    await flush();

    expect(fetchInitial).toHaveBeenCalledTimes(2);
    expect(fetchInitial).toHaveBeenLastCalledWith('s1', 'agent-a1.jsonl', true);
    expect(adapter.fetchIncremental).not.toHaveBeenCalled();
    expect(result.current.notFound).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.items).toEqual(['x', 'y']);
  });

  it('keeps non-404 errors as errors without flagging notFound', async () => {
    const fetchInitial = vi.fn().mockRejectedValueOnce(new APIError('Boom', 500, 'Server Error'));
    const adapter = makeAdapter({ fetchInitial });
    const { result } = renderHook(() => useTranscriptData(adapter, 's1', 'transcript.jsonl', undefined));
    await flush();
    expect(result.current.error).toBe('Boom');
    expect(result.current.notFound).toBe(false);
  });

  it('drops the previous file\'s data immediately when fileName changes', async () => {
    const fetchInitial = vi
      .fn()
      .mockResolvedValueOnce({ items: ['main'], raw: ['main'], totalLines: 1 })
      .mockReturnValueOnce(new Promise(() => {}));
    const adapter = makeAdapter({ fetchInitial });
    const { result, rerender } = renderHook(
      ({ fileName }: { fileName: string }) => useTranscriptData(adapter, 's1', fileName, undefined),
      { initialProps: { fileName: 'agent-a1.jsonl' } },
    );
    await flush();
    expect(result.current.items).toEqual(['main']);

    rerender({ fileName: 'agent-a2.jsonl' });
    expect(result.current.items).toEqual([]);
    expect(result.current.loading).toBe(true);
  });

  it('does not append a stale poll result after fileName changes', async () => {
    let resolvePoll: (v: unknown) => void = () => {};
    const fetchIncremental = vi.fn().mockReturnValueOnce(
      new Promise((resolve) => {
        resolvePoll = resolve;
      }),
    );
    const fetchInitial = vi
      .fn()
      .mockResolvedValueOnce({ items: ['a'], raw: ['a'], totalLines: 1 })
      .mockResolvedValueOnce({ items: ['b'], raw: ['b'], totalLines: 1 });
    const adapter = makeAdapter({ fetchInitial, fetchIncremental });
    const { result, rerender } = renderHook(
      ({ fileName }: { fileName: string }) => useTranscriptData(adapter, 's1', fileName, undefined),
      { initialProps: { fileName: 'agent-a1.jsonl' } },
    );
    await flush();
    await act(async () => {
      vi.advanceTimersByTime(POLL_MS);
    });

    rerender({ fileName: 'agent-a2.jsonl' });
    await flush();
    await act(async () => {
      resolvePoll({ newItems: ['stale'], newRaw: ['stale'], newTotalLineCount: 2 });
    });
    await flush();

    expect(result.current.items).toEqual(['b']);
  });

  it('follows seed changes without fetching', () => {
    const adapter = makeAdapter();
    const seedA = { raw: ['a'] };
    const seedB = { raw: ['b', 'c'] };
    const { result, rerender } = renderHook(
      ({ seed }: { seed: { raw: unknown[] } }) => useTranscriptData(adapter, 's1', 'f.jsonl', seed),
      { initialProps: { seed: seedA } },
    );
    expect(result.current.items).toEqual(['a']);
    rerender({ seed: seedB });
    expect(result.current.items).toEqual(['b', 'c']);
    expect(adapter.fetchInitial).not.toHaveBeenCalled();
  });
});
