import { useCallback } from 'react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';

export interface SessionFilters {
  repos: string[];
  branches: string[];
  owners: string[];
  providers: string[];
  query: string;
}

interface SessionFiltersActions {
  toggleRepo: (value: string) => void;
  toggleBranch: (value: string) => void;
  toggleOwner: (value: string) => void;
  toggleProvider: (value: string) => void;
  setQuery: (value: string) => void;
  clearAll: () => void;
  commitHistory: () => void;
}

const SESSION_FILTERS_CONFIG: URLFiltersConfig = {
  repos: { type: 'string[]', default: [], paramName: 'repo' },
  branches: { type: 'string[]', default: [], paramName: 'branch' },
  owners: { type: 'string[]', default: [], paramName: 'owner' },
  providers: { type: 'string[]', default: [], paramName: 'provider' },
  query: { type: 'string', default: '', paramName: 'q' },
};

export function useSessionFilters(): SessionFilters & SessionFiltersActions {
  const { filters, toggleArrayValue, setFilter, clearAll, commitHistory } =
    useURLFilters<SessionFilters>(SESSION_FILTERS_CONFIG);

  const toggleRepo = useCallback(
    (value: string) => toggleArrayValue('repos', value),
    [toggleArrayValue],
  );

  const toggleBranch = useCallback(
    (value: string) => toggleArrayValue('branches', value),
    [toggleArrayValue],
  );

  const toggleOwner = useCallback(
    (value: string) => toggleArrayValue('owners', value),
    [toggleArrayValue],
  );

  const toggleProvider = useCallback(
    (value: string) => toggleArrayValue('providers', value),
    [toggleArrayValue],
  );

  const setQuery = useCallback(
    (value: string) => setFilter('query', value, { replace: true }),
    [setFilter],
  );

  return {
    ...filters,
    toggleRepo,
    toggleBranch,
    toggleOwner,
    toggleProvider,
    setQuery,
    clearAll,
    commitHistory,
  };
}
