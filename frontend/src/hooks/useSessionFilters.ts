import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo, useState } from 'react';
import type { SortDirection } from '@/utils';

type SortColumn = 'summary' | 'external_id' | 'last_sync_time';

interface SessionFilters {
  showSharedWithMe: boolean;
  selectedRepo: string | null;
  selectedBranch: string | null;
  sortColumn: SortColumn;
  sortDirection: SortDirection;
  showEmptySessions: boolean;
}

interface SessionFiltersActions {
  setShowSharedWithMe: (value: boolean) => void;
  setSelectedRepo: (value: string | null) => void;
  setSelectedBranch: (value: string | null) => void;
  setSortColumn: (value: SortColumn) => void;
  setSortDirection: (value: SortDirection) => void;
  handleSort: (column: SortColumn) => void;
  handleRepoClick: (repo: string | null) => void;
  toggleShowEmptySessions: () => void;
}

const PARAM_KEYS = {
  shared: 'shared',
  repo: 'repo',
  branch: 'branch',
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
    const sharedParam = searchParams.get(PARAM_KEYS.shared);
    const repoParam = searchParams.get(PARAM_KEYS.repo);
    const branchParam = searchParams.get(PARAM_KEYS.branch);
    const sortParam = searchParams.get(PARAM_KEYS.sort);
    const dirParam = searchParams.get(PARAM_KEYS.dir);

    return {
      showSharedWithMe: sharedParam === '1',
      selectedRepo: repoParam,
      selectedBranch: branchParam,
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

  const setShowSharedWithMe = useCallback(
    (value: boolean) => {
      updateParams({
        [PARAM_KEYS.shared]: value ? '1' : null,
        // Reset repo/branch filters when switching tabs
        [PARAM_KEYS.repo]: null,
        [PARAM_KEYS.branch]: null,
      });
    },
    [updateParams]
  );

  const setSelectedRepo = useCallback(
    (value: string | null) => {
      updateParams({
        [PARAM_KEYS.repo]: value,
        [PARAM_KEYS.branch]: null, // Reset branch when repo changes
      });
    },
    [updateParams]
  );

  const setSelectedBranch = useCallback(
    (value: string | null) => {
      updateParams({ [PARAM_KEYS.branch]: value });
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
        [PARAM_KEYS.branch]: null, // Reset branch when repo changes
      });
    },
    [updateParams]
  );

  const toggleShowEmptySessions = useCallback(() => {
    setShowEmptySessions((prev) => !prev);
  }, []);

  return {
    ...filters,
    setShowSharedWithMe,
    setSelectedRepo,
    setSelectedBranch,
    setSortColumn,
    setSortDirection,
    handleSort,
    handleRepoClick,
    toggleShowEmptySessions,
  };
}
