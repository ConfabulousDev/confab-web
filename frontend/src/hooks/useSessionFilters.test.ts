import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSessionFilters } from './useSessionFilters';

// Track calls to setSearchParams for verification
const mockSetSearchParams = vi.fn();
let currentParams = new URLSearchParams();

vi.mock('react-router-dom', () => ({
  useSearchParams: () => [currentParams, mockSetSearchParams],
}));

function setParams(params: Record<string, string>) {
  currentParams = new URLSearchParams(params);
}

describe('useSessionFilters', () => {
  beforeEach(() => {
    currentParams = new URLSearchParams();
    mockSetSearchParams.mockClear();
    // Make setSearchParams apply the callback so we can inspect results
    mockSetSearchParams.mockImplementation((updater: (prev: URLSearchParams) => URLSearchParams) => {
      if (typeof updater === 'function') {
        currentParams = updater(currentParams);
      }
    });
  });

  describe('initial state', () => {
    it('returns empty filters when URL has no params', () => {
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.repos).toEqual([]);
      expect(result.current.branches).toEqual([]);
      expect(result.current.owners).toEqual([]);
      expect(result.current.query).toBe('');
    });

    it('parses comma-separated repo values from URL', () => {
      setParams({ repo: 'confab-web,confab-cli' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.repos).toEqual(['confab-web', 'confab-cli']);
    });

    it('parses comma-separated branch values from URL', () => {
      setParams({ branch: 'main,develop' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.branches).toEqual(['main', 'develop']);
    });

    it('parses comma-separated owner values from URL', () => {
      setParams({ owner: 'alice@co.com,bob@co.com' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.owners).toEqual(['alice@co.com', 'bob@co.com']);
    });

    it('parses query from URL', () => {
      setParams({ q: 'fix auth bug' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.query).toBe('fix auth bug');
    });
  });

  describe('toggleRepo', () => {
    it('adds repo when not present', () => {
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleRepo('confab-web'));

      expect(mockSetSearchParams).toHaveBeenCalledTimes(1);
      // Verify the resulting params
      expect(currentParams.get('repo')).toBe('confab-web');
    });

    it('removes repo when already present', () => {
      setParams({ repo: 'confab-web,confab-cli' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleRepo('confab-web'));

      expect(currentParams.get('repo')).toBe('confab-cli');
    });

    it('clears repo param when last repo is removed', () => {
      setParams({ repo: 'confab-web' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleRepo('confab-web'));

      expect(currentParams.has('repo')).toBe(false);
    });
  });

  describe('toggleBranch', () => {
    it('adds branch when not present', () => {
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleBranch('main'));

      expect(currentParams.get('branch')).toBe('main');
    });

    it('removes branch when already present', () => {
      setParams({ branch: 'main,develop' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleBranch('main'));

      expect(currentParams.get('branch')).toBe('develop');
    });
  });

  describe('toggleOwner', () => {
    it('adds owner when not present', () => {
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleOwner('alice@co.com'));

      expect(currentParams.get('owner')).toBe('alice@co.com');
    });

    it('removes owner when already present', () => {
      setParams({ owner: 'alice@co.com' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleOwner('alice@co.com'));

      expect(currentParams.has('owner')).toBe(false);
    });
  });

  describe('setQuery', () => {
    it('sets query param', () => {
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.setQuery('fix bug'));

      expect(currentParams.get('q')).toBe('fix bug');
    });

    it('removes query param when empty', () => {
      setParams({ q: 'old query' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.setQuery(''));

      expect(currentParams.has('q')).toBe(false);
    });
  });

  describe('clearAll', () => {
    it('clears all params', () => {
      setParams({ repo: 'confab-web', branch: 'main', owner: 'alice@co.com', q: 'test' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.clearAll());

      expect(mockSetSearchParams).toHaveBeenCalledWith({}, { replace: true });
    });
  });
});
