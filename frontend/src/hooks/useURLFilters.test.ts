import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';
import type { DateRange } from '@/utils/dateRange';

const mockSetSearchParams = vi.fn();
let currentParams = new URLSearchParams();

vi.mock('react-router-dom', () => ({
  useSearchParams: () => [currentParams, mockSetSearchParams],
}));

function setParams(params: Record<string, string>) {
  currentParams = new URLSearchParams(params);
}

// Apply the functional updater so we can inspect the resulting params
function applyUpdater() {
  const call = mockSetSearchParams.mock.calls[mockSetSearchParams.mock.calls.length - 1]!;
  const updater = call[0];
  if (typeof updater === 'function') {
    currentParams = updater(currentParams);
  } else if (updater instanceof URLSearchParams) {
    currentParams = updater;
  } else {
    currentParams = new URLSearchParams(updater);
  }
}

describe('useURLFilters', () => {
  beforeEach(() => {
    currentParams = new URLSearchParams();
    mockSetSearchParams.mockClear();
  });

  // --- String field ---

  describe('string fields', () => {
    const config: URLFiltersConfig = {
      query: { type: 'string', default: '', paramName: 'q' },
    };

    it('returns default when param absent', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ query: string }>(config),
      );
      expect(result.current.filters.query).toBe('');
    });

    it('reads value from URL', () => {
      setParams({ q: 'hello' });
      const { result } = renderHook(() =>
        useURLFilters<{ query: string }>(config),
      );
      expect(result.current.filters.query).toBe('hello');
    });

    it('sets a string value in URL', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ query: string }>(config),
      );
      act(() => result.current.setFilter('query', 'search term'));
      applyUpdater();
      expect(currentParams.get('q')).toBe('search term');
    });

    it('removes param when set to default', () => {
      setParams({ q: 'old' });
      const { result } = renderHook(() =>
        useURLFilters<{ query: string }>(config),
      );
      act(() => result.current.setFilter('query', ''));
      applyUpdater();
      expect(currentParams.has('q')).toBe(false);
    });
  });

  // --- String array field ---

  describe('string[] fields', () => {
    const config: URLFiltersConfig = {
      repos: { type: 'string[]', default: [], paramName: 'repo' },
    };

    it('returns default when param absent', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      expect(result.current.filters.repos).toEqual([]);
    });

    it('parses comma-separated values', () => {
      setParams({ repo: 'web,cli' });
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      expect(result.current.filters.repos).toEqual(['web', 'cli']);
    });

    it('handles single value (no comma)', () => {
      setParams({ repo: 'web' });
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      expect(result.current.filters.repos).toEqual(['web']);
    });

    it('sets array value in URL', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      act(() => result.current.setFilter('repos', ['web', 'cli']));
      applyUpdater();
      expect(currentParams.get('repo')).toBe('web,cli');
    });

    it('removes param when set to default (empty array)', () => {
      setParams({ repo: 'web' });
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      act(() => result.current.setFilter('repos', []));
      applyUpdater();
      expect(currentParams.has('repo')).toBe(false);
    });
  });

  // --- toggleArrayValue ---

  describe('toggleArrayValue', () => {
    const config: URLFiltersConfig = {
      repos: { type: 'string[]', default: [], paramName: 'repo' },
    };

    it('adds value when not present', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      act(() => result.current.toggleArrayValue('repos', 'web'));
      applyUpdater();
      expect(currentParams.get('repo')).toBe('web');
    });

    it('removes value when already present', () => {
      setParams({ repo: 'web,cli' });
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      act(() => result.current.toggleArrayValue('repos', 'web'));
      applyUpdater();
      expect(currentParams.get('repo')).toBe('cli');
    });

    it('clears param when last value removed', () => {
      setParams({ repo: 'web' });
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      act(() => result.current.toggleArrayValue('repos', 'web'));
      applyUpdater();
      expect(currentParams.has('repo')).toBe(false);
    });

    it('is a no-op for non-array fields', () => {
      const mixedConfig: URLFiltersConfig = {
        query: { type: 'string', default: '', paramName: 'q' },
      };
      const { result } = renderHook(() =>
        useURLFilters<{ query: string }>(mixedConfig),
      );
      act(() => result.current.toggleArrayValue('query', 'val'));
      expect(mockSetSearchParams).not.toHaveBeenCalled();
    });
  });

  // --- Boolean field ---

  describe('boolean fields', () => {
    const config: URLFiltersConfig = {
      includeNoRepo: { type: 'boolean', default: true, paramName: 'includeNoRepo' },
    };

    it('returns default when param absent', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ includeNoRepo: boolean }>(config),
      );
      expect(result.current.filters.includeNoRepo).toBe(true);
    });

    it('parses "false" from URL', () => {
      setParams({ includeNoRepo: 'false' });
      const { result } = renderHook(() =>
        useURLFilters<{ includeNoRepo: boolean }>(config),
      );
      expect(result.current.filters.includeNoRepo).toBe(false);
    });

    it('parses "true" from URL', () => {
      setParams({ includeNoRepo: 'true' });
      const { result } = renderHook(() =>
        useURLFilters<{ includeNoRepo: boolean }>(config),
      );
      expect(result.current.filters.includeNoRepo).toBe(true);
    });

    it('removes param when set to default', () => {
      setParams({ includeNoRepo: 'false' });
      const { result } = renderHook(() =>
        useURLFilters<{ includeNoRepo: boolean }>(config),
      );
      act(() => result.current.setFilter('includeNoRepo', true));
      applyUpdater();
      expect(currentParams.has('includeNoRepo')).toBe(false);
    });

    it('sets param when non-default', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ includeNoRepo: boolean }>(config),
      );
      act(() => result.current.setFilter('includeNoRepo', false));
      applyUpdater();
      expect(currentParams.get('includeNoRepo')).toBe('false');
    });
  });

  // --- DateRange field ---

  describe('dateRange fields', () => {
    const defaultRange = { startDate: '2026-03-16', endDate: '2026-03-22', label: 'Last 7 Days' };
    const config: URLFiltersConfig = {
      dateRange: {
        type: 'dateRange',
        default: defaultRange,
        paramName: { start: 'start', end: 'end' },
      },
    };

    type Filters = { dateRange: DateRange };

    it('returns default when params absent', () => {
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      expect(result.current.filters.dateRange).toEqual(defaultRange);
    });

    it('parses dates from URL', () => {
      setParams({ start: '2026-01-01', end: '2026-01-31' });
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      expect(result.current.filters.dateRange.startDate).toBe('2026-01-01');
      expect(result.current.filters.dateRange.endDate).toBe('2026-01-31');
    });

    it('returns default for invalid date format', () => {
      setParams({ start: 'bad', end: '2026-01-31' });
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      expect(result.current.filters.dateRange).toEqual(defaultRange);
    });

    it('returns default when only one date param present', () => {
      setParams({ start: '2026-01-01' });
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      expect(result.current.filters.dateRange).toEqual(defaultRange);
    });

    it('removes date params when set to default', () => {
      setParams({ start: '2026-01-01', end: '2026-01-31' });
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      act(() => result.current.setFilter('dateRange', defaultRange));
      applyUpdater();
      expect(currentParams.has('start')).toBe(false);
      expect(currentParams.has('end')).toBe(false);
    });

    it('sets date params for non-default range', () => {
      const custom: DateRange = { startDate: '2026-02-01', endDate: '2026-02-28', label: 'Feb' };
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      act(() => result.current.setFilter('dateRange', custom));
      applyUpdater();
      expect(currentParams.get('start')).toBe('2026-02-01');
      expect(currentParams.get('end')).toBe('2026-02-28');
    });
  });

  // --- clearAll ---

  describe('clearAll', () => {
    it('nukes all search params', () => {
      setParams({ repo: 'web', q: 'test', start: '2026-01-01' });
      const config: URLFiltersConfig = {
        repos: { type: 'string[]', default: [], paramName: 'repo' },
      };
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[] }>(config),
      );
      act(() => result.current.clearAll());
      expect(mockSetSearchParams).toHaveBeenCalledWith({}, { replace: true });
    });
  });

  // --- History behavior ---

  describe('history behavior', () => {
    const config: URLFiltersConfig = {
      repos: { type: 'string[]', default: [], paramName: 'repo' },
      query: { type: 'string', default: '', paramName: 'q' },
    };

    it('defaults to push (replace: false)', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[]; query: string }>(config),
      );
      act(() => result.current.setFilter('query', 'test'));
      expect(mockSetSearchParams).toHaveBeenCalledWith(
        expect.any(Function),
        { replace: false },
      );
    });

    it('respects replace: true option', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[]; query: string }>(config),
      );
      act(() => result.current.setFilter('query', 'test', { replace: true }));
      expect(mockSetSearchParams).toHaveBeenCalledWith(
        expect.any(Function),
        { replace: true },
      );
    });

    it('toggleArrayValue defaults to push', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[]; query: string }>(config),
      );
      act(() => result.current.toggleArrayValue('repos', 'web'));
      expect(mockSetSearchParams).toHaveBeenCalledWith(
        expect.any(Function),
        { replace: false },
      );
    });

    it('toggleArrayValue respects replace option', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[]; query: string }>(config),
      );
      act(() => result.current.toggleArrayValue('repos', 'web', { replace: true }));
      expect(mockSetSearchParams).toHaveBeenCalledWith(
        expect.any(Function),
        { replace: true },
      );
    });

    it('commitHistory pushes current state', () => {
      setParams({ repo: 'web' });
      const { result } = renderHook(() =>
        useURLFilters<{ repos: string[]; query: string }>(config),
      );
      act(() => result.current.commitHistory());
      expect(mockSetSearchParams).toHaveBeenCalledWith(
        expect.any(Function),
        { replace: false },
      );
    });
  });

  // --- Non-empty default arrays ---

  describe('non-empty default arrays', () => {
    const config: URLFiltersConfig = {
      hide: {
        type: 'string[]',
        default: ['system', 'summary'],
        paramName: 'hide',
      },
    };

    it('returns default when param absent', () => {
      const { result } = renderHook(() =>
        useURLFilters<{ hide: string[] }>(config),
      );
      expect(result.current.filters.hide).toEqual(['system', 'summary']);
    });

    it('reads explicit value from URL (overriding default)', () => {
      setParams({ hide: 'thinking' });
      const { result } = renderHook(() =>
        useURLFilters<{ hide: string[] }>(config),
      );
      expect(result.current.filters.hide).toEqual(['thinking']);
    });

    it('removes param when set back to default', () => {
      setParams({ hide: 'thinking' });
      const { result } = renderHook(() =>
        useURLFilters<{ hide: string[] }>(config),
      );
      act(() => result.current.setFilter('hide', ['system', 'summary']));
      applyUpdater();
      expect(currentParams.has('hide')).toBe(false);
    });

    it('removes param when set to default in different order', () => {
      setParams({ hide: 'thinking' });
      const { result } = renderHook(() =>
        useURLFilters<{ hide: string[] }>(config),
      );
      act(() => result.current.setFilter('hide', ['summary', 'system']));
      applyUpdater();
      expect(currentParams.has('hide')).toBe(false);
    });
  });

  // --- Multiple fields ---

  describe('multiple fields', () => {
    const config: URLFiltersConfig = {
      repos: { type: 'string[]', default: [], paramName: 'repo' },
      query: { type: 'string', default: '', paramName: 'q' },
      includeNoRepo: { type: 'boolean', default: true, paramName: 'includeNoRepo' },
    };

    type Filters = { repos: string[]; query: string; includeNoRepo: boolean };

    it('reads all fields from URL', () => {
      setParams({ repo: 'web,cli', q: 'search', includeNoRepo: 'false' });
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      expect(result.current.filters).toEqual({
        repos: ['web', 'cli'],
        query: 'search',
        includeNoRepo: false,
      });
    });

    it('setting one field preserves others', () => {
      setParams({ repo: 'web', q: 'search' });
      const { result } = renderHook(() => useURLFilters<Filters>(config));
      act(() => result.current.setFilter('query', 'new search'));
      applyUpdater();
      expect(currentParams.get('repo')).toBe('web');
      expect(currentParams.get('q')).toBe('new search');
    });
  });
});
