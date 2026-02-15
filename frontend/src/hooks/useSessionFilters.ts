import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo } from 'react';

export interface SessionFilters {
  repos: string[];
  branches: string[];
  owners: string[];
  query: string;
}

export interface SessionFiltersActions {
  toggleRepo: (value: string) => void;
  toggleBranch: (value: string) => void;
  toggleOwner: (value: string) => void;
  setQuery: (value: string) => void;
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
      query: searchParams.get('q') || '',
    };
  }, [searchParams]);

  const updateParams = useCallback(
    (updates: Record<string, string | null>) => {
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

  const setQuery = useCallback(
    (value: string) => {
      updateParams({ q: value || null });
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
    setQuery,
    clearAll,
  };
}
