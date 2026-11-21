import { useCallback } from 'react';
import { useURLFilters, type URLFiltersConfig } from './useURLFilters';

export interface SessionFilters {
  repos: string[];
  branches: string[];
  owners: string[];
  providers: string[];
  query: string;
  /** xtrf: 'all' = admin "All sessions" scope; '' = default (mine & shared).
   *  Optional so callers that build filters by hand can omit it. */
  scope?: SessionScope;
}

export type SessionScope = '' | 'all';

interface SessionFiltersActions {
  toggleRepo: (value: string) => void;
  toggleBranch: (value: string) => void;
  toggleOwner: (value: string) => void;
  toggleProvider: (value: string) => void;
  setQuery: (value: string) => void;
  setScope: (value: SessionScope) => void;
  /** Clears every filter chip; keeps the scope, which is a separate control. */
  clearAll: () => void;
  commitHistory: () => void;
}

const SESSION_FILTERS_CONFIG: URLFiltersConfig = {
  repos: { type: 'string[]', default: [], paramName: 'repo' },
  branches: { type: 'string[]', default: [], paramName: 'branch' },
  owners: { type: 'string[]', default: [], paramName: 'owner' },
  providers: { type: 'string[]', default: [], paramName: 'provider' },
  query: { type: 'string', default: '', paramName: 'q' },
  scope: { type: 'string', default: '', paramName: 'scope' },
};

const CLEARED_FILTERS: Partial<SessionFilters> = {
  repos: [],
  branches: [],
  owners: [],
  providers: [],
  query: '',
};

export function useSessionFilters(): SessionFilters & SessionFiltersActions {
  const { filters, toggleArrayValue, setFilter, setAll, commitHistory } =
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

  const setScope = useCallback(
    (value: SessionScope) => setFilter('scope', value),
    [setFilter],
  );

  const clearAll = useCallback(
    () => setAll(CLEARED_FILTERS, { replace: true }),
    [setAll],
  );

  return {
    ...filters,
    scope: filters.scope === 'all' ? 'all' : '',
    toggleRepo,
    toggleBranch,
    toggleOwner,
    toggleProvider,
    setQuery,
    setScope,
    clearAll,
    commitHistory,
  };
}
