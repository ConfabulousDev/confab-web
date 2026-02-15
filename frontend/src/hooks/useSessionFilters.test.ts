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
      expect(result.current.page).toBe(1);
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

    it('parses page from URL', () => {
      setParams({ page: '3' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.page).toBe(3);
    });

    it('defaults page to 1 for invalid values', () => {
      setParams({ page: 'abc' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.page).toBe(1);
    });

    it('clamps page to minimum 1', () => {
      setParams({ page: '0' });
      const { result } = renderHook(() => useSessionFilters());
      expect(result.current.page).toBe(1);
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

    it('resets page when toggling repo', () => {
      setParams({ repo: 'confab-web', page: '3' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleRepo('confab-cli'));

      expect(currentParams.has('page')).toBe(false);
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

    it('resets page when changing query', () => {
      setParams({ page: '3' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.setQuery('search'));

      expect(currentParams.has('page')).toBe(false);
    });
  });

  describe('setPage', () => {
    it('sets page param for page > 1', () => {
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.setPage(2));

      expect(currentParams.get('page')).toBe('2');
    });

    it('removes page param for page 1', () => {
      setParams({ page: '3' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.setPage(1));

      expect(currentParams.has('page')).toBe(false);
    });

    it('does not reset page (setPage uses resetPage=false)', () => {
      setParams({ repo: 'confab-web' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.setPage(5));

      // repo should still be present
      expect(currentParams.get('repo')).toBe('confab-web');
      expect(currentParams.get('page')).toBe('5');
    });
  });

  describe('clearAll', () => {
    it('clears all params', () => {
      setParams({ repo: 'confab-web', branch: 'main', owner: 'alice@co.com', q: 'test', page: '2' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.clearAll());

      expect(mockSetSearchParams).toHaveBeenCalledWith({}, { replace: true });
    });
  });

  describe('filter changes reset page', () => {
    it('toggleRepo resets page', () => {
      setParams({ page: '5', repo: 'a' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleRepo('b'));
      expect(currentParams.has('page')).toBe(false);
    });

    it('toggleBranch resets page', () => {
      setParams({ page: '5' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleBranch('main'));
      expect(currentParams.has('page')).toBe(false);
    });

    it('toggleOwner resets page', () => {
      setParams({ page: '5' });
      const { result } = renderHook(() => useSessionFilters());
      act(() => result.current.toggleOwner('alice@co.com'));
      expect(currentParams.has('page')).toBe(false);
    });

  });
});
