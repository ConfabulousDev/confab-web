import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo } from 'react';

export interface SessionFilters {
  repos: string[];
  branches: string[];
  owners: string[];
  prs: string[];
  query: string;
  page: number;
}

export interface SessionFiltersActions {
  toggleRepo: (value: string) => void;
  toggleBranch: (value: string) => void;
  toggleOwner: (value: string) => void;
  addPR: (value: string) => void;
  removePR: (value: string) => void;
  setQuery: (value: string) => void;
  setPage: (page: number) => void;
  clearAll: () => void;
}

function parseCommaSeparated(value: string | null): string[] {
  if (!value) return [];
  return value.split(',').filter(Boolean);
}

function joinOrEmpty(values: string[]): string | null {
  return values.length > 0 ? values.join(',') : null;
}

export function useSessionFilters(): SessionFilters & SessionFiltersActions {
  const [searchParams, setSearchParams] = useSearchParams();

  const filters = useMemo<SessionFilters>(() => {
    return {
      repos: parseCommaSeparated(searchParams.get('repo')),
      branches: parseCommaSeparated(searchParams.get('branch')),
      owners: parseCommaSeparated(searchParams.get('owner')),
      prs: parseCommaSeparated(searchParams.get('pr')),
      query: searchParams.get('q') || '',
      page: Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1),
    };
  }, [searchParams]);

  const updateParams = useCallback(
    (updates: Record<string, string | null>, resetPage = true) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [key, value] of Object.entries(updates)) {
            if (value === null || value === '') {
              next.delete(key);
            } else {
              next.set(key, value);
            }
          }
          if (resetPage) {
            next.delete('page');
          }
          return next;
        },
        { replace: true }
      );
    },
    [setSearchParams]
  );

  const toggleRepo = useCallback(
    (value: string) => {
      const current = parseCommaSeparated(searchParams.get('repo'));
      const next = current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value];
      updateParams({ repo: joinOrEmpty(next) });
    },
    [searchParams, updateParams]
  );

  const toggleBranch = useCallback(
    (value: string) => {
      const current = parseCommaSeparated(searchParams.get('branch'));
      const next = current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value];
      updateParams({ branch: joinOrEmpty(next) });
    },
    [searchParams, updateParams]
  );

  const toggleOwner = useCallback(
    (value: string) => {
      const current = parseCommaSeparated(searchParams.get('owner'));
      const next = current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value];
      updateParams({ owner: joinOrEmpty(next) });
    },
    [searchParams, updateParams]
  );

  const addPR = useCallback(
    (value: string) => {
      const current = parseCommaSeparated(searchParams.get('pr'));
      if (current.includes(value)) return;
      updateParams({ pr: joinOrEmpty([...current, value]) });
    },
    [searchParams, updateParams]
  );

  const removePR = useCallback(
    (value: string) => {
      const current = parseCommaSeparated(searchParams.get('pr'));
      const next = current.filter((v) => v !== value);
      updateParams({ pr: joinOrEmpty(next) });
    },
    [searchParams, updateParams]
  );

  const setQuery = useCallback(
    (value: string) => {
      updateParams({ q: value || null });
    },
    [updateParams]
  );

  const setPage = useCallback(
    (page: number) => {
      updateParams({ page: page > 1 ? String(page) : null }, false);
    },
    [updateParams]
  );

  const clearAll = useCallback(() => {
    setSearchParams({}, { replace: true });
  }, [setSearchParams]);

  return {
    ...filters,
    toggleRepo,
    toggleBranch,
    toggleOwner,
    addPR,
    removePR,
    setQuery,
    setPage,
    clearAll,
  };
}
