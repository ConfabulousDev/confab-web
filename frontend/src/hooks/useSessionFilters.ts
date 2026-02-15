import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo, useState } from 'react';
import type { SortDirection } from '@/utils';

type SortColumn = 'summary' | 'external_id' | 'last_sync_time';

interface SessionFilters {
  selectedRepo: string | null;
  selectedBranch: string | null;
  selectedHostname: string | null;
  selectedOwner: string | null;
  selectedPR: string | null;
  selectedCommit: string | null;
  searchQuery: string;
  sortColumn: SortColumn;
  sortDirection: SortDirection;
  showEmptySessions: boolean;
}

interface SessionFiltersActions {
  setSelectedBranch: (value: string | null) => void;
  setSelectedPR: (value: string | null) => void;
  setSelectedCommit: (value: string | null) => void;
  setSearchQuery: (value: string) => void;
  setSortColumn: (value: SortColumn) => void;
  setSortDirection: (value: SortDirection) => void;
  handleSort: (column: SortColumn) => void;
  handleRepoClick: (repo: string | null) => void;
  handleHostnameClick: (hostname: string | null) => void;
  handleOwnerClick: (owner: string | null) => void;
  toggleShowEmptySessions: () => void;
}

const PARAM_KEYS = {
  repo: 'repo',
  branch: 'branch',
  hostname: 'hostname',
  owner: 'owner',
  pr: 'pr',
  commit: 'commit',
  q: 'q',
  sort: 'sort',
  dir: 'dir',
} as const;

const DEFAULT_SORT_COLUMN: SortColumn = 'last_sync_time';
const DEFAULT_SORT_DIRECTION: SortDirection = 'desc';

function isValidSortColumn(value: string | null): value is SortColumn {
  return value === 'summary' || value === 'external_id' || value === 'last_sync_time';
}

function isValidSortDirection(value: string | null): value is SortDirection {
  return value === 'asc' || value === 'desc';
}

export function useSessionFilters(): SessionFilters & SessionFiltersActions {
  const [searchParams, setSearchParams] = useSearchParams();

  // Not persisted to URL - hidden dev feature
  const [showEmptySessions, setShowEmptySessions] = useState(false);

  const filters = useMemo<SessionFilters>(() => {
    const repoParam = searchParams.get(PARAM_KEYS.repo);
    const branchParam = searchParams.get(PARAM_KEYS.branch);
    const hostnameParam = searchParams.get(PARAM_KEYS.hostname);
    const ownerParam = searchParams.get(PARAM_KEYS.owner);
    const prParam = searchParams.get(PARAM_KEYS.pr);
    const commitParam = searchParams.get(PARAM_KEYS.commit);
    const queryParam = searchParams.get(PARAM_KEYS.q);
    const sortParam = searchParams.get(PARAM_KEYS.sort);
    const dirParam = searchParams.get(PARAM_KEYS.dir);

    return {
      selectedRepo: repoParam,
      selectedBranch: branchParam,
      selectedHostname: hostnameParam,
      selectedOwner: ownerParam,
      selectedPR: prParam,
      selectedCommit: commitParam,
      searchQuery: queryParam || '',
      sortColumn: isValidSortColumn(sortParam) ? sortParam : DEFAULT_SORT_COLUMN,
      sortDirection: isValidSortDirection(dirParam) ? dirParam : DEFAULT_SORT_DIRECTION,
      showEmptySessions,
    };
  }, [searchParams, showEmptySessions]);

  const updateParams = useCallback(
    (updates: Partial<Record<keyof typeof PARAM_KEYS, string | null>>) => {
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

  const setSelectedBranch = useCallback(
    (value: string | null) => {
      updateParams({ [PARAM_KEYS.branch]: value });
    },
    [updateParams]
  );

  const setSelectedPR = useCallback(
    (value: string | null) => {
      updateParams({ [PARAM_KEYS.pr]: value });
    },
    [updateParams]
  );

  const setSelectedCommit = useCallback(
    (value: string | null) => {
      updateParams({ [PARAM_KEYS.commit]: value });
    },
    [updateParams]
  );

  const setSearchQuery = useCallback(
    (value: string) => {
      updateParams({ [PARAM_KEYS.q]: value || null });
    },
    [updateParams]
  );

  const setSortColumn = useCallback(
    (value: SortColumn) => {
      updateParams({
        [PARAM_KEYS.sort]: value === DEFAULT_SORT_COLUMN ? null : value,
      });
    },
    [updateParams]
  );

  const setSortDirection = useCallback(
    (value: SortDirection) => {
      updateParams({
        [PARAM_KEYS.dir]: value === DEFAULT_SORT_DIRECTION ? null : value,
      });
    },
    [updateParams]
  );

  const handleSort = useCallback(
    (column: SortColumn) => {
      if (filters.sortColumn === column) {
        const newDir = filters.sortDirection === 'asc' ? 'desc' : 'asc';
        updateParams({
          [PARAM_KEYS.dir]: newDir === DEFAULT_SORT_DIRECTION ? null : newDir,
        });
      } else {
        const defaultDir = column === 'last_sync_time' ? 'desc' : 'asc';
        updateParams({
          [PARAM_KEYS.sort]: column === DEFAULT_SORT_COLUMN ? null : column,
          [PARAM_KEYS.dir]: defaultDir === DEFAULT_SORT_DIRECTION ? null : defaultDir,
        });
      }
    },
    [filters.sortColumn, filters.sortDirection, updateParams]
  );

  const handleRepoClick = useCallback(
    (repo: string | null) => {
      updateParams({
        [PARAM_KEYS.repo]: repo,
        // Reset repo-scoped filters when repo changes
        [PARAM_KEYS.branch]: null,
        [PARAM_KEYS.pr]: null,
        [PARAM_KEYS.commit]: null,
      });
    },
    [updateParams]
  );

  const handleHostnameClick = useCallback(
    (hostname: string | null) => {
      updateParams({ [PARAM_KEYS.hostname]: hostname });
    },
    [updateParams]
  );

  const handleOwnerClick = useCallback(
    (owner: string | null) => {
      updateParams({ [PARAM_KEYS.owner]: owner });
    },
    [updateParams]
  );

  const toggleShowEmptySessions = useCallback(() => {
    setShowEmptySessions((prev) => !prev);
  }, []);

  return {
    ...filters,
    setSelectedBranch,
    setSelectedPR,
    setSelectedCommit,
    setSearchQuery,
    setSortColumn,
    setSortDirection,
    handleSort,
    handleRepoClick,
    handleHostnameClick,
    handleOwnerClick,
    toggleShowEmptySessions,
  };
}
