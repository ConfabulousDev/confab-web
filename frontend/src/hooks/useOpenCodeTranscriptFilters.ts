// x5w2: URL-synced filter hook for OpenCode transcripts.
//
// Parallel to the Claude/Codex transcript filter hooks, built on the shared
// useProviderTranscriptFilters. OpenCode categories are flat (no hierarchy):
// `user`, `assistant`, `tool`, `unknown`. A token appears in the shared `?hide=`
// URL slot iff its FilterState boolean is `false` (hidden); since every category
// is visible by default, DEFAULT_HIDDEN is empty and a default view carries no
// `?hide=` param. Foreign-provider tokens are ignored on read.

import {
  useProviderTranscriptFilters,
  type ProviderTranscriptFiltersConfig,
} from './useProviderTranscriptFilters';
import {
  DEFAULT_OPENCODE_FILTER_STATE,
  type OpenCodeCategory,
  type OpenCodeFilterState,
} from '@/components/session/opencodeCategories';

const FLAT_KEYS = ['user', 'assistant', 'tool', 'unknown'] as const satisfies readonly OpenCodeCategory[];

export function pathsFromState(state: OpenCodeFilterState): string[] {
  return FLAT_KEYS.filter((key) => !state[key]);
}

export function stateFromPaths(paths: string[]): OpenCodeFilterState {
  const hidden = new Set(paths);
  return {
    user: !hidden.has('user'),
    assistant: !hidden.has('assistant'),
    tool: !hidden.has('tool'),
    unknown: !hidden.has('unknown'),
  };
}

export const DEFAULT_HIDDEN: string[] = pathsFromState(DEFAULT_OPENCODE_FILTER_STATE);

const CONFIG = {
  defaultState: DEFAULT_OPENCODE_FILTER_STATE,
  pathsFromState,
  stateFromPaths,
  hierarchicalKeys: {},
} satisfies ProviderTranscriptFiltersConfig<OpenCodeFilterState>;

interface OpenCodeTranscriptFiltersResult {
  filterState: OpenCodeFilterState;
  setFilterState: (state: OpenCodeFilterState, opts?: { replace?: boolean }) => void;
  toggleCategory: (category: OpenCodeCategory) => void;
}

export function useOpenCodeTranscriptFilters(): OpenCodeTranscriptFiltersResult {
  const { filterState, setFilterState, toggleCategory } =
    useProviderTranscriptFilters<OpenCodeFilterState>(CONFIG);
  return { filterState, setFilterState, toggleCategory };
}
