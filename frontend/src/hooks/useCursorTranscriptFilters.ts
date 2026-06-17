// URL-synced filter hook for Cursor transcripts (18n2).
//
// Parallel to the OpenCode transcript filter hook (x5w2 pattern), built on the
// shared useProviderTranscriptFilters. Cursor categories are flat (no
// hierarchy): `user`, `assistant`, `tool`. A token appears in the shared
// `?hide=` URL slot iff its FilterState boolean is `false` (hidden); every
// category is visible by default, so DEFAULT_HIDDEN is empty and a default view
// carries no `?hide=` param. Foreign-provider tokens are ignored on read.

import {
  useProviderTranscriptFilters,
  type ProviderTranscriptFiltersConfig,
} from './useProviderTranscriptFilters';
import {
  DEFAULT_CURSOR_FILTER_STATE,
  type CursorCategory,
  type CursorFilterState,
} from '@/components/session/cursorCategories';

const FLAT_KEYS = ['user', 'assistant', 'tool'] as const satisfies readonly CursorCategory[];

export function pathsFromState(state: CursorFilterState): string[] {
  return FLAT_KEYS.filter((key) => !state[key]);
}

export function stateFromPaths(paths: string[]): CursorFilterState {
  const hidden = new Set(paths);
  return {
    user: !hidden.has('user'),
    assistant: !hidden.has('assistant'),
    tool: !hidden.has('tool'),
  };
}

export const DEFAULT_HIDDEN: string[] = pathsFromState(DEFAULT_CURSOR_FILTER_STATE);

const CONFIG = {
  defaultState: DEFAULT_CURSOR_FILTER_STATE,
  pathsFromState,
  stateFromPaths,
  hierarchicalKeys: {},
} satisfies ProviderTranscriptFiltersConfig<CursorFilterState>;

interface CursorTranscriptFiltersResult {
  filterState: CursorFilterState;
  setFilterState: (state: CursorFilterState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: CursorCategory) => void;
}

export function useCursorTranscriptFilters(): CursorTranscriptFiltersResult {
  const { filterState, setFilterState, toggleCategory } =
    useProviderTranscriptFilters<CursorFilterState>(CONFIG);
  return { filterState, setFilterState, toggleCategory };
}
